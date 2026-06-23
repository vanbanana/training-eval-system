package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteTaskRepo struct {
	db *store.DB
}

func NewTaskRepo(db *store.DB) TaskRepo {
	return &SQLiteTaskRepo{db: db}
}

func (r *SQLiteTaskRepo) GetByID(ctx context.Context, id int64) (*model.TrainingTask, error) {
	var t model.TrainingTask
	var deadline, createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, name, description, requirements, evaluation_criteria, teacher_id, course_id,
		        status, deadline, created_at, updated_at
		 FROM training_tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Name, &t.Description, &t.Requirements, &t.EvaluationCriteria,
		&t.TeacherID, &t.CourseID, &t.Status, &deadline, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task_repo: not found")
		}
		return nil, fmt.Errorf("task_repo: get: %w", err)
	}
	t.Deadline = parseNullTime(deadline)
	t.CreatedAt = parseTime(createdAt.String)
	t.UpdatedAt = parseTime(updatedAt.String)

	// Load dimensions
	dims, err := r.GetDimensions(ctx, id)
	if err == nil {
		t.Dimensions = dims
	}

	// Load class IDs
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT class_id FROM task_classes WHERE task_id=?", id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int64
			if rows.Scan(&cid) == nil {
				t.ClassIDs = append(t.ClassIDs, cid)
			}
		}
	}

	return &t, nil
}

func (r *SQLiteTaskRepo) List(ctx context.Context, params TaskListParams) ([]model.TrainingTask, int64, error) {
	where := "1=1"
	args := []any{}

	if params.TeacherID != nil {
		where += " AND teacher_id=?"
		args = append(args, *params.TeacherID)
	}
	if params.CourseID != nil {
		where += " AND course_id=?"
		args = append(args, *params.CourseID)
	}
	if params.Status != nil {
		where += " AND status=?"
		args = append(args, *params.Status)
	}
	if params.Search != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+params.Search+"%")
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM training_tasks WHERE %s", where)
	if err := r.db.Reader.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("task_repo: count: %w", err)
	}

	orderBy := "id DESC"
	if params.SortBy != "" {
		dir := "ASC"
		if params.SortDir == "desc" {
			dir = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, dir)
	}

	querySQL := fmt.Sprintf(
		`SELECT id, name, description, requirements, evaluation_criteria, teacher_id, course_id,
		        status, deadline, created_at, updated_at
		 FROM training_tasks WHERE %s ORDER BY %s LIMIT ? OFFSET ?`, where, orderBy)
	args = append(args, params.PageSize, params.Offset())

	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("task_repo: list: %w", err)
	}
	defer rows.Close()

	var tasks []model.TrainingTask
	for rows.Next() {
		var t model.TrainingTask
		var deadline, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Requirements, &t.EvaluationCriteria,
			&t.TeacherID, &t.CourseID, &t.Status, &deadline, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("task_repo: scan: %w", err)
		}
		t.Deadline = parseNullTime(deadline)
		t.CreatedAt = parseTime(createdAt.String)
		t.UpdatedAt = parseTime(updatedAt.String)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Batch-load dimensions for all listed tasks so list responses carry the
	// dimension set (previously empty, making the UI show "0 个维度").
	if err := r.attachDimensions(ctx, tasks); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// attachDimensions loads dimensions for the given tasks in a single query and
// assigns them by task ID, avoiding an N+1 query per task.
func (r *SQLiteTaskRepo) attachDimensions(ctx context.Context, tasks []model.TrainingTask) error {
	if len(tasks) == 0 {
		return nil
	}
	placeholders := make([]string, len(tasks))
	args := make([]any, len(tasks))
	for i := range tasks {
		placeholders[i] = "?"
		args[i] = tasks[i].ID
	}
	query := fmt.Sprintf(
		`SELECT id, task_id, name, description, weight, order_index FROM dimensions
		 WHERE task_id IN (%s) ORDER BY task_id, order_index`,
		strings.Join(placeholders, ","))
	rows, err := r.db.Reader.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("task_repo: list dimensions: %w", err)
	}
	defer rows.Close()

	byTask := make(map[int64][]model.Dimension)
	for rows.Next() {
		var d model.Dimension
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Name, &d.Description, &d.Weight, &d.OrderIndex); err != nil {
			return err
		}
		byTask[d.TaskID] = append(byTask[d.TaskID], d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range tasks {
		tasks[i].Dimensions = byTask[tasks[i].ID]
	}
	return nil
}

func (r *SQLiteTaskRepo) Create(ctx context.Context, t *model.TrainingTask) error {
	var deadlineStr *string
	if t.Deadline != nil {
		s := t.Deadline.Format("2006-01-02 15:04:05")
		deadlineStr = &s
	}
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO training_tasks (name, description, requirements, evaluation_criteria, teacher_id, course_id, status, deadline, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		t.Name, t.Description, t.Requirements, t.EvaluationCriteria, t.TeacherID, t.CourseID, t.Status, deadlineStr)
	if err != nil {
		return fmt.Errorf("task_repo: create: %w", err)
	}
	id, _ := res.LastInsertId()
	t.ID = id
	return nil
}

func (r *SQLiteTaskRepo) Update(ctx context.Context, t *model.TrainingTask) error {
	var deadlineStr *string
	if t.Deadline != nil {
		s := t.Deadline.Format("2006-01-02 15:04:05")
		deadlineStr = &s
	}
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE training_tasks SET name=?, description=?, requirements=?, evaluation_criteria=?,
		 course_id=?, status=?, deadline=?, updated_at=datetime('now') WHERE id=?`,
		t.Name, t.Description, t.Requirements, t.EvaluationCriteria, t.CourseID, t.Status, deadlineStr, t.ID)
	if err != nil {
		return fmt.Errorf("task_repo: update: %w", err)
	}
	return nil
}

func (r *SQLiteTaskRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM training_tasks WHERE id=?", id)
	return err
}

func (r *SQLiteTaskRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE training_tasks SET status=?, updated_at=datetime('now') WHERE id=?", status, id)
	return err
}

func (r *SQLiteTaskRepo) SetClasses(ctx context.Context, taskID int64, classIDs []int64) error {
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM task_classes WHERE task_id=?", taskID)
	for _, cid := range classIDs {
		if _, err := r.db.Writer.ExecContext(ctx,
			"INSERT INTO task_classes (task_id, class_id) VALUES (?, ?)", taskID, cid); err != nil {
			return fmt.Errorf("task_repo: set classes: %w", err)
		}
	}
	return nil
}

func (r *SQLiteTaskRepo) GetDimensions(ctx context.Context, taskID int64) ([]model.Dimension, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id, task_id, name, description, weight, order_index FROM dimensions WHERE task_id=? ORDER BY order_index", taskID)
	if err != nil {
		return nil, fmt.Errorf("task_repo: get dimensions: %w", err)
	}
	defer rows.Close()

	var dims []model.Dimension
	for rows.Next() {
		var d model.Dimension
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Name, &d.Description, &d.Weight, &d.OrderIndex); err != nil {
			return nil, err
		}
		dims = append(dims, d)
	}
	return dims, rows.Err()
}

func (r *SQLiteTaskRepo) SetDimensions(ctx context.Context, taskID int64, dims []model.Dimension) error {
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM dimensions WHERE task_id=?", taskID)
	for i, d := range dims {
		res, err := r.db.Writer.ExecContext(ctx,
			"INSERT INTO dimensions (task_id, name, description, weight, order_index) VALUES (?, ?, ?, ?, ?)",
			taskID, d.Name, d.Description, d.Weight, i)
		if err != nil {
			return fmt.Errorf("task_repo: set dimensions: %w", err)
		}
		id, _ := res.LastInsertId()
		dims[i].ID = id
	}
	return nil
}
