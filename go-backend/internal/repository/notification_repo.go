package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteNotificationRepo struct {
	db *store.DB
}

func NewNotificationRepo(db *store.DB) NotificationRepo {
	return &SQLiteNotificationRepo{db: db}
}

func (r *SQLiteNotificationRepo) GetByID(ctx context.Context, id int64) (*model.Notification, error) {
	var n model.Notification
	var isRead int
	var payloadStr sql.NullString
	var createdAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, user_id, type, title, content, payload, is_read, link, created_at
		 FROM notifications WHERE id=?`, id).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &payloadStr, &isRead, &n.Link, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification_repo: not found")
		}
		return nil, err
	}
	n.IsRead = isRead != 0
	if payloadStr.Valid {
		_ = json.Unmarshal([]byte(payloadStr.String), &n.Payload)
	}
	n.CreatedAt = parseTime(createdAt.String)
	return &n, nil
}

func (r *SQLiteNotificationRepo) List(ctx context.Context, userID int64, unreadOnly bool, params ListParams) ([]model.Notification, int64, error) {
	where := "user_id=?"
	args := []any{userID}
	if unreadOnly {
		where += " AND is_read=0"
	}

	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM notifications WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	querySQL := fmt.Sprintf(
		`SELECT id, user_id, type, title, content, payload, is_read, link, created_at
		 FROM notifications WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, where)
	args = append(args, params.PageSize, params.Offset())

	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifs []model.Notification
	for rows.Next() {
		var n model.Notification
		var isRead int
		var payloadStr sql.NullString
		var createdAt sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &payloadStr, &isRead, &n.Link, &createdAt); err != nil {
			return nil, 0, err
		}
		n.IsRead = isRead != 0
		if payloadStr.Valid {
			_ = json.Unmarshal([]byte(payloadStr.String), &n.Payload)
		}
		n.CreatedAt = parseTime(createdAt.String)
		notifs = append(notifs, n)
	}
	return notifs, total, rows.Err()
}

func (r *SQLiteNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	payloadJSON, _ := json.Marshal(n.Payload)
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO notifications (user_id, type, title, content, payload, is_read, link)
		 VALUES (?, ?, ?, ?, ?, 0, ?)`,
		n.UserID, n.Type, n.Title, n.Content, string(payloadJSON), n.Link)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	n.ID = id
	return nil
}

func (r *SQLiteNotificationRepo) MarkRead(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "UPDATE notifications SET is_read=1 WHERE id=?", id)
	return err
}

func (r *SQLiteNotificationRepo) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "UPDATE notifications SET is_read=1 WHERE user_id=? AND is_read=0", userID)
	return err
}

func (r *SQLiteNotificationRepo) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id=? AND is_read=0", userID).Scan(&count)
	return count, err
}

func (r *SQLiteNotificationRepo) GetPreferencesByUserID(ctx context.Context, userID int64) ([]model.NotificationPref, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id, user_id, event_type, enabled FROM notification_prefs WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prefs []model.NotificationPref
	for rows.Next() {
		var p model.NotificationPref
		var enabled int
		if err := rows.Scan(&p.ID, &p.UserID, &p.EventType, &enabled); err != nil {
			return nil, err
		}
		p.Enabled = enabled != 0
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

func (r *SQLiteNotificationRepo) UpsertPreference(ctx context.Context, pref *model.NotificationPref) error {
	enabled := 0
	if pref.Enabled {
		enabled = 1
	}
	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO notification_prefs (user_id, event_type, enabled)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id, event_type) DO UPDATE SET enabled=excluded.enabled`,
		pref.UserID, pref.EventType, enabled)
	return err
}
