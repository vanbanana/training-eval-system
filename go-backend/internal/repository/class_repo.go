package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteClassRepo struct{ db *store.DB }

func NewClassRepo(db *store.DB) ClassRepo { return &SQLiteClassRepo{db: db} }

func (r *SQLiteClassRepo) GetByID(ctx context.Context, id int64) (*model.Class, error) {
	var c model.Class
	var isArchived int
	var createdAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,name,course_id,teacher_id,student_count,is_archived,created_at FROM classes WHERE id=?", id).Scan(
		&c.ID, &c.Name, &c.CourseID, &c.TeacherID, &c.StudentCount, &isArchived, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("class_repo: not found")
		}
		return nil, err
	}
	c.IsArchived = isArchived != 0
	c.CreatedAt = parseTime(createdAt.String)
	return &c, nil
}

func (r *SQLiteClassRepo) List(ctx context.Context, courseID *int64, teacherID *int64) ([]model.Class, error) {
	where := "1=1"
	args := []any{}
	if courseID != nil {
		where += " AND course_id=?"
		args = append(args, *courseID)
	}
	if teacherID != nil {
		where += " AND teacher_id=?"
		args = append(args, *teacherID)
	}
	rows, err := r.db.Reader.QueryContext(ctx,
		fmt.Sprintf("SELECT id,name,course_id,teacher_id,student_count,is_archived,created_at FROM classes WHERE %s ORDER BY id", where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var classes []model.Class
	for rows.Next() {
		var c model.Class
		var isArchived int
		var createdAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.CourseID, &c.TeacherID, &c.StudentCount, &isArchived, &createdAt); err != nil {
			return nil, err
		}
		c.IsArchived = isArchived != 0
		c.CreatedAt = parseTime(createdAt.String)
		classes = append(classes, c)
	}
	return classes, rows.Err()
}

func (r *SQLiteClassRepo) Create(ctx context.Context, c *model.Class) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO classes (name,course_id,teacher_id,student_count,is_archived) VALUES (?,?,?,?,?)",
		c.Name, c.CourseID, c.TeacherID, c.StudentCount, boolToInt(c.IsArchived))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return nil
}

func (r *SQLiteClassRepo) Update(ctx context.Context, c *model.Class) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE classes SET name=?,is_archived=? WHERE id=?", c.Name, boolToInt(c.IsArchived), c.ID)
	return err
}

func (r *SQLiteClassRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM classes WHERE id=?", id)
	return err
}

func (r *SQLiteClassRepo) AddMember(ctx context.Context, classID, studentID int64) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"INSERT OR IGNORE INTO class_memberships (class_id,student_id) VALUES (?,?)", classID, studentID)
	if err != nil {
		return err
	}
	_, _ = r.db.Writer.ExecContext(ctx,
		"UPDATE classes SET student_count=(SELECT COUNT(*) FROM class_memberships WHERE class_id=?) WHERE id=?", classID, classID)
	return nil
}

func (r *SQLiteClassRepo) RemoveMember(ctx context.Context, classID, studentID int64) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"DELETE FROM class_memberships WHERE class_id=? AND student_id=?", classID, studentID)
	if err != nil {
		return err
	}
	_, _ = r.db.Writer.ExecContext(ctx,
		"UPDATE classes SET student_count=(SELECT COUNT(*) FROM class_memberships WHERE class_id=?) WHERE id=?", classID, classID)
	return nil
}

func (r *SQLiteClassRepo) GetMembers(ctx context.Context, classID int64) ([]model.ClassMembership, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id,class_id,student_id,joined_at FROM class_memberships WHERE class_id=?", classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []model.ClassMembership
	for rows.Next() {
		var m model.ClassMembership
		var joinedAt sql.NullString
		if err := rows.Scan(&m.ID, &m.ClassID, &m.StudentID, &joinedAt); err != nil {
			return nil, err
		}
		m.JoinedAt = parseTime(joinedAt.String)
		members = append(members, m)
	}
	return members, rows.Err()
}
