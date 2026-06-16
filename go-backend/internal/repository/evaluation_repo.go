package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteEvaluationRepo struct {
	db *store.DB
}

func NewEvaluationRepo(db *store.DB) EvaluationRepo {
	return &SQLiteEvaluationRepo{db: db}
}

func (r *SQLiteEvaluationRepo) GetByID(ctx context.Context, id int64) (*model.Evaluation, error) {
	var e model.Evaluation
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, task_id, student_id, upload_id, status, total_score, objective_ratio, teacher_comment, overall_comment, created_at, updated_at
		 FROM evaluations WHERE id=?`, id).Scan(
		&e.ID, &e.TaskID, &e.StudentID, &e.UploadID, &e.Status, &e.TotalScore, &e.ObjectiveRatio, &e.TeacherComment, &e.OverallComment, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("evaluation_repo: not found")
		}
		return nil, fmt.Errorf("evaluation_repo: get: %w", err)
	}
	e.CreatedAt = parseTime(createdAt.String)
	e.UpdatedAt = parseTime(updatedAt.String)

	// Load scores
	scores, _ := r.getScores(ctx, id)
	e.Scores = scores
	return &e, nil
}

func (r *SQLiteEvaluationRepo) List(ctx context.Context, params EvalListParams) ([]model.Evaluation, int64, error) {
	where := "1=1"
	args := []any{}

	if params.TaskID != nil {
		where += " AND task_id=?"
		args = append(args, *params.TaskID)
	}
	if params.StudentID != nil {
		where += " AND student_id=?"
		args = append(args, *params.StudentID)
	}
	if params.UploadID != nil {
		where += " AND upload_id=?"
		args = append(args, *params.UploadID)
	}
	if params.Status != nil {
		where += " AND status=?"
		args = append(args, *params.Status)
	}

	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM evaluations WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	querySQL := fmt.Sprintf(
		`SELECT id, task_id, student_id, upload_id, status, total_score, objective_ratio, teacher_comment, overall_comment, created_at, updated_at
		 FROM evaluations WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, params.PageSize, params.Offset())

	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var evals []model.Evaluation
	for rows.Next() {
		var e model.Evaluation
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&e.ID, &e.TaskID, &e.StudentID, &e.UploadID, &e.Status,
			&e.TotalScore, &e.ObjectiveRatio, &e.TeacherComment, &e.OverallComment, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		e.CreatedAt = parseTime(createdAt.String)
		e.UpdatedAt = parseTime(updatedAt.String)
		// Load dimension scores for each evaluation
		scores, _ := r.getScores(ctx, e.ID)
		e.Scores = scores
		evals = append(evals, e)
	}
	return evals, total, rows.Err()
}

func (r *SQLiteEvaluationRepo) Create(ctx context.Context, e *model.Evaluation) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO evaluations (task_id, student_id, upload_id, status, total_score, objective_ratio, teacher_comment, overall_comment, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		e.TaskID, e.StudentID, e.UploadID, e.Status, e.TotalScore, e.ObjectiveRatio, e.TeacherComment, e.OverallComment)
	if err != nil {
		return fmt.Errorf("evaluation_repo: create: %w", err)
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return nil
}

func (r *SQLiteEvaluationRepo) Update(ctx context.Context, e *model.Evaluation) error {
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE evaluations SET status=?, total_score=?, objective_ratio=?, teacher_comment=?, overall_comment=?, updated_at=datetime('now') WHERE id=?`,
		e.Status, e.TotalScore, e.ObjectiveRatio, e.TeacherComment, e.OverallComment, e.ID)
	return err
}

func (r *SQLiteEvaluationRepo) Delete(ctx context.Context, id int64) error {
	// Delete associated dimension scores first
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM dimension_scores WHERE evaluation_id=?", id)
	// Delete evaluation histories
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM evaluation_histories WHERE evaluation_id=?", id)
	// Delete the evaluation itself
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM evaluations WHERE id=?", id)
	return err
}

func (r *SQLiteEvaluationRepo) BatchConfirm(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if _, err := r.db.Writer.ExecContext(ctx,
			"UPDATE evaluations SET status='confirmed', updated_at=datetime('now') WHERE id=?", id); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteEvaluationRepo) SaveScores(ctx context.Context, evalID int64, scores []model.DimensionScore) error {
	// Delete existing scores
	_, _ = r.db.Writer.ExecContext(ctx, "DELETE FROM dimension_scores WHERE evaluation_id=?", evalID)

	for _, s := range scores {
		_, err := r.db.Writer.ExecContext(ctx,
			`INSERT INTO dimension_scores (evaluation_id, dimension_id, ai_score, teacher_score, rationale)
			 VALUES (?, ?, ?, ?, ?)`,
			evalID, s.DimensionID, s.AIScore, s.TeacherScore, s.Rationale)
		if err != nil {
			return fmt.Errorf("evaluation_repo: save scores: %w", err)
		}
	}
	return nil
}

func (r *SQLiteEvaluationRepo) AppendHistory(ctx context.Context, h *model.EvaluationHistory) error {
	beforeJSON, _ := json.Marshal(h.BeforeValue)
	afterJSON, _ := json.Marshal(h.AfterValue)
	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO evaluation_histories (evaluation_id, operator_id, action, before_value, after_value, changed_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		h.EvaluationID, h.OperatorID, h.Action, string(beforeJSON), string(afterJSON))
	return err
}

func (r *SQLiteEvaluationRepo) GetHistory(ctx context.Context, evalID int64) ([]model.EvaluationHistory, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, evaluation_id, operator_id, action, before_value, after_value, changed_at
		 FROM evaluation_histories WHERE evaluation_id=? ORDER BY changed_at DESC`, evalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.EvaluationHistory
	for rows.Next() {
		var h model.EvaluationHistory
		var beforeJSON, afterJSON sql.NullString
		var changedAt sql.NullString
		if err := rows.Scan(&h.ID, &h.EvaluationID, &h.OperatorID, &h.Action, &beforeJSON, &afterJSON, &changedAt); err != nil {
			return nil, err
		}
		if beforeJSON.Valid && beforeJSON.String != "" {
			json.Unmarshal([]byte(beforeJSON.String), &h.BeforeValue)
		}
		if afterJSON.Valid && afterJSON.String != "" {
			json.Unmarshal([]byte(afterJSON.String), &h.AfterValue)
		}
		h.ChangedAt = parseTime(changedAt.String)
		items = append(items, h)
	}
	return items, rows.Err()
}

func (r *SQLiteEvaluationRepo) getScores(ctx context.Context, evalID int64) ([]model.DimensionScore, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, evaluation_id, dimension_id, ai_score, teacher_score, rationale
		 FROM dimension_scores WHERE evaluation_id=?`, evalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []model.DimensionScore
	for rows.Next() {
		var s model.DimensionScore
		if err := rows.Scan(&s.ID, &s.EvaluationID, &s.DimensionID, &s.AIScore, &s.TeacherScore, &s.Rationale); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	return scores, rows.Err()
}
