package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteTemplateRepo struct{ db *store.DB }

func NewTemplateRepo(db *store.DB) TemplateRepo { return &SQLiteTemplateRepo{db: db} }

func (r *SQLiteTemplateRepo) GetByID(ctx context.Context, id int64) (*model.EvalTemplate, error) {
	var t model.EvalTemplate
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,name,description,visibility,owner_id,course_id,created_at,updated_at FROM eval_templates WHERE id=?", id).Scan(
		&t.ID, &t.Name, &t.Description, &t.Visibility, &t.OwnerID, &t.CourseID, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template_repo: not found")
		}
		return nil, err
	}
	t.CreatedAt = parseTime(createdAt.String)
	t.UpdatedAt = parseTime(updatedAt.String)
	items, _ := r.getItems(ctx, id)
	t.Items = items
	return &t, nil
}

func (r *SQLiteTemplateRepo) List(ctx context.Context, ownerID *int64, courseID *int64, visibility *string) ([]model.EvalTemplate, error) {
	where := "1=1"
	args := []any{}
	if ownerID != nil {
		where += " AND owner_id=?"
		args = append(args, *ownerID)
	}
	if courseID != nil {
		where += " AND course_id=?"
		args = append(args, *courseID)
	}
	if visibility != nil {
		where += " AND visibility=?"
		args = append(args, *visibility)
	}
	rows, err := r.db.Reader.QueryContext(ctx,
		fmt.Sprintf("SELECT id,name,description,visibility,owner_id,course_id,created_at,updated_at FROM eval_templates WHERE %s ORDER BY id DESC", where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []model.EvalTemplate
	for rows.Next() {
		var t model.EvalTemplate
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Visibility, &t.OwnerID, &t.CourseID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(createdAt.String)
		t.UpdatedAt = parseTime(updatedAt.String)
		items, _ := r.getItems(ctx, t.ID)
		t.Items = items
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (r *SQLiteTemplateRepo) Create(ctx context.Context, t *model.EvalTemplate) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO eval_templates (name,description,visibility,owner_id,course_id) VALUES (?,?,?,?,?)",
		t.Name, t.Description, t.Visibility, t.OwnerID, t.CourseID)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	t.ID = id
	return nil
}

func (r *SQLiteTemplateRepo) Update(ctx context.Context, t *model.EvalTemplate) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE eval_templates SET name=?,description=?,visibility=?,updated_at=datetime('now') WHERE id=?",
		t.Name, t.Description, t.Visibility, t.ID)
	return err
}

func (r *SQLiteTemplateRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM eval_templates WHERE id=?", id)
	return err
}

func (r *SQLiteTemplateRepo) SetItems(ctx context.Context, templateID int64, items []model.TemplateDimension) error {
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM template_dimensions WHERE template_id=?", templateID)
	for i, item := range items {
		_, err := r.db.Writer.ExecContext(ctx,
			"INSERT INTO template_dimensions (template_id,name,description,weight,order_index) VALUES (?,?,?,?,?)",
			templateID, item.Name, item.Description, item.Weight, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteTemplateRepo) getItems(ctx context.Context, templateID int64) ([]model.TemplateDimension, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id,template_id,name,description,weight,order_index FROM template_dimensions WHERE template_id=? ORDER BY order_index", templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.TemplateDimension
	for rows.Next() {
		var d model.TemplateDimension
		if err := rows.Scan(&d.ID, &d.TemplateID, &d.Name, &d.Description, &d.Weight, &d.OrderIndex); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
