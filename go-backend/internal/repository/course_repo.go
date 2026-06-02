package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteCourseRepo struct{ db *store.DB }

func NewCourseRepo(db *store.DB) CourseRepo { return &SQLiteCourseRepo{db: db} }

func (r *SQLiteCourseRepo) GetByID(ctx context.Context, id int64) (*model.Course, error) {
	var c model.Course
	var isArchived int
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,name,code,is_archived,created_at,updated_at FROM courses WHERE id=?", id).Scan(
		&c.ID, &c.Name, &c.Code, &isArchived, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("course_repo: not found")
		}
		return nil, err
	}
	c.IsArchived = isArchived != 0
	c.CreatedAt = parseTime(createdAt.String)
	c.UpdatedAt = parseTime(updatedAt.String)
	return &c, nil
}

func (r *SQLiteCourseRepo) List(ctx context.Context, params ListParams) ([]model.Course, int64, error) {
	where := "1=1"
	args := []any{}
	if params.Search != "" {
		where += " AND (name LIKE ? OR code LIKE ?)"
		like := "%" + params.Search + "%"
		args = append(args, like, like)
	}
	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM courses WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	querySQL := fmt.Sprintf("SELECT id,name,code,is_archived,created_at,updated_at FROM courses WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?", where)
	args = append(args, params.PageSize, params.Offset())
	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var courses []model.Course
	for rows.Next() {
		var c model.Course
		var isArchived int
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &isArchived, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		c.IsArchived = isArchived != 0
		c.CreatedAt = parseTime(createdAt.String)
		c.UpdatedAt = parseTime(updatedAt.String)
		courses = append(courses, c)
	}
	return courses, total, rows.Err()
}

func (r *SQLiteCourseRepo) Create(ctx context.Context, c *model.Course) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO courses (name,code,is_archived) VALUES (?,?,?)", c.Name, c.Code, boolToInt(c.IsArchived))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return nil
}

func (r *SQLiteCourseRepo) Update(ctx context.Context, c *model.Course) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE courses SET name=?,code=?,is_archived=?,updated_at=datetime('now') WHERE id=?",
		c.Name, c.Code, boolToInt(c.IsArchived), c.ID)
	return err
}

func (r *SQLiteCourseRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM courses WHERE id=?", id)
	return err
}
