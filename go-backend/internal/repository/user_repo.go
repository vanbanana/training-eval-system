package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

// SQLiteUserRepo implements UserRepo using SQLite.
type SQLiteUserRepo struct {
	db *store.DB
}

// NewUserRepo creates a new SQLite-backed user repository.
func NewUserRepo(db *store.DB) UserRepo {
	return &SQLiteUserRepo{db: db}
}

func (r *SQLiteUserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return r.scanUser(r.db.Reader.QueryRowContext(ctx,
		`SELECT id, username, display_name, password_hash, role, is_active,
		        failed_login_count, locked_until, last_login_at, created_at, updated_at
		 FROM users WHERE id = ?`, id))
}

func (r *SQLiteUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return r.scanUser(r.db.Reader.QueryRowContext(ctx,
		`SELECT id, username, display_name, password_hash, role, is_active,
		        failed_login_count, locked_until, last_login_at, created_at, updated_at
		 FROM users WHERE username = ?`, username))
}

func (r *SQLiteUserRepo) List(ctx context.Context, params ListParams) ([]model.User, int64, error) {
	where := "1=1"
	args := []any{}

	if params.Search != "" {
		where += " AND (username LIKE ? OR display_name LIKE ?)"
		like := "%" + params.Search + "%"
		args = append(args, like, like)
	}

	// Count
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", where)
	if err := r.db.Reader.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("user_repo: count: %w", err)
	}

	// Query
	orderBy := "id ASC"
	if params.SortBy != "" {
		dir := "ASC"
		if params.SortDir == "desc" {
			dir = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, dir)
	}

	querySQL := fmt.Sprintf(
		`SELECT id, username, display_name, password_hash, role, is_active,
		        failed_login_count, locked_until, last_login_at, created_at, updated_at
		 FROM users WHERE %s ORDER BY %s LIMIT ? OFFSET ?`,
		where, orderBy)

	args = append(args, params.PageSize, params.Offset())
	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("user_repo: list: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *u)
	}
	return users, total, rows.Err()
}

func (r *SQLiteUserRepo) Create(ctx context.Context, u *model.User) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO users (username, display_name, password_hash, role, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		u.Username, u.DisplayName, u.PasswordHash, u.Role, boolToInt(u.IsActive))
	if err != nil {
		return fmt.Errorf("user_repo: create: %w", err)
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (r *SQLiteUserRepo) Update(ctx context.Context, u *model.User) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE users SET display_name=?, role=?, is_active=?, password_hash=?, updated_at=datetime('now')
		 WHERE id=?`,
		u.DisplayName, u.Role, boolToInt(u.IsActive), u.PasswordHash, u.ID)
	if err != nil {
		return fmt.Errorf("user_repo: update: %w", err)
	}
	return nil
}

func (r *SQLiteUserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("user_repo: delete: %w", err)
	}
	return nil
}

func (r *SQLiteUserRepo) UpdateLoginState(ctx context.Context, id int64, failed int, lockedUntil *time.Time) error {
	var lockStr *string
	if lockedUntil != nil {
		s := lockedUntil.Format(time.RFC3339)
		lockStr = &s
	}
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE users SET failed_login_count=?, locked_until=?, updated_at=datetime('now') WHERE id=?`,
		failed, lockStr, id)
	if err != nil {
		return fmt.Errorf("user_repo: update login state: %w", err)
	}
	return nil
}

func (r *SQLiteUserRepo) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE users SET last_login_at=?, updated_at=? WHERE id=?`, now, now, id)
	if err != nil {
		return fmt.Errorf("user_repo: update last login: %w", err)
	}
	return nil
}

func (r *SQLiteUserRepo) ToggleActive(ctx context.Context, id int64, active bool) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE users SET is_active=?, updated_at=datetime('now') WHERE id=?`,
		boolToInt(active), id)
	if err != nil {
		return fmt.Errorf("user_repo: toggle active: %w", err)
	}
	return nil
}

// --- scan helpers ---

func (r *SQLiteUserRepo) scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var isActive int
	var lockedUntil, lastLogin, createdAt, updatedAt sql.NullString

	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role,
		&isActive, &u.FailedLoginCount, &lockedUntil, &lastLogin, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user_repo: not found")
		}
		return nil, fmt.Errorf("user_repo: scan: %w", err)
	}

	u.IsActive = isActive != 0
	u.LockedUntil = parseNullTime(lockedUntil)
	u.LastLoginAt = parseNullTime(lastLogin)
	u.CreatedAt = parseTime(createdAt.String)
	u.UpdatedAt = parseTime(updatedAt.String)
	return &u, nil
}

func (r *SQLiteUserRepo) scanUserFromRows(rows *sql.Rows) (*model.User, error) {
	var u model.User
	var isActive int
	var lockedUntil, lastLogin, createdAt, updatedAt sql.NullString

	err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role,
		&isActive, &u.FailedLoginCount, &lockedUntil, &lastLogin, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("user_repo: scan row: %w", err)
	}

	u.IsActive = isActive != 0
	u.LockedUntil = parseNullTime(lockedUntil)
	u.LastLoginAt = parseNullTime(lastLogin)
	u.CreatedAt = parseTime(createdAt.String)
	u.UpdatedAt = parseTime(updatedAt.String)
	return &u, nil
}

// --- utility functions ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	if t.IsZero() {
		return nil
	}
	return &t
}

func parseTime(s string) time.Time {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Try SQLite datetime format (stored as UTC)
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC()
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
