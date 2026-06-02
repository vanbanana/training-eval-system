package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteSimilarityRepo struct{ db *store.DB }

func NewSimilarityRepo(db *store.DB) SimilarityRepo { return &SQLiteSimilarityRepo{db: db} }

func (r *SQLiteSimilarityRepo) GetByID(ctx context.Context, id int64) (*model.SimilarityRecord, error) {
	var s model.SimilarityRecord
	var createdAt, decidedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id,task_id,upload_a_id,upload_b_id,hamming_distance,cosine_similarity,state,reviewed_by,created_at,decided_at
		 FROM similarity_records WHERE id=?`, id).Scan(
		&s.ID, &s.TaskID, &s.UploadAID, &s.UploadBID, &s.HammingDistance,
		&s.CosineSimilarity, &s.State, &s.ReviewedBy, &createdAt, &decidedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("similarity_repo: not found")
		}
		return nil, err
	}
	s.CreatedAt = parseTime(createdAt.String)
	s.DecidedAt = parseNullTime(decidedAt)
	return &s, nil
}

func (r *SQLiteSimilarityRepo) List(ctx context.Context, taskID int64, state *string) ([]model.SimilarityRecord, error) {
	where := "task_id=?"
	args := []any{taskID}
	if state != nil {
		where += " AND state=?"
		args = append(args, *state)
	}
	rows, err := r.db.Reader.QueryContext(ctx,
		fmt.Sprintf(`SELECT id,task_id,upload_a_id,upload_b_id,hamming_distance,cosine_similarity,state,reviewed_by,created_at,decided_at
		 FROM similarity_records WHERE %s ORDER BY id DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []model.SimilarityRecord
	for rows.Next() {
		var s model.SimilarityRecord
		var createdAt, decidedAt sql.NullString
		if err := rows.Scan(&s.ID, &s.TaskID, &s.UploadAID, &s.UploadBID, &s.HammingDistance,
			&s.CosineSimilarity, &s.State, &s.ReviewedBy, &createdAt, &decidedAt); err != nil {
			return nil, err
		}
		s.CreatedAt = parseTime(createdAt.String)
		s.DecidedAt = parseNullTime(decidedAt)
		records = append(records, s)
	}
	return records, rows.Err()
}

func (r *SQLiteSimilarityRepo) Create(ctx context.Context, rec *model.SimilarityRecord) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO similarity_records (task_id,upload_a_id,upload_b_id,hamming_distance,cosine_similarity,state)
		 VALUES (?,?,?,?,?,?)`,
		rec.TaskID, rec.UploadAID, rec.UploadBID, rec.HammingDistance, rec.CosineSimilarity, rec.State)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	return nil
}

func (r *SQLiteSimilarityRepo) UpdateState(ctx context.Context, id int64, state string, reviewedBy int64) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE similarity_records SET state=?,reviewed_by=?,decided_at=? WHERE id=?", state, reviewedBy, now, id)
	return err
}

func (r *SQLiteSimilarityRepo) GetByTaskPair(ctx context.Context, taskID, uploadAID, uploadBID int64) (*model.SimilarityRecord, error) {
	var s model.SimilarityRecord
	var createdAt, decidedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id,task_id,upload_a_id,upload_b_id,hamming_distance,cosine_similarity,state,reviewed_by,created_at,decided_at
		 FROM similarity_records WHERE task_id=? AND upload_a_id=? AND upload_b_id=?`, taskID, uploadAID, uploadBID).Scan(
		&s.ID, &s.TaskID, &s.UploadAID, &s.UploadBID, &s.HammingDistance,
		&s.CosineSimilarity, &s.State, &s.ReviewedBy, &createdAt, &decidedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.CreatedAt = parseTime(createdAt.String)
	s.DecidedAt = parseNullTime(decidedAt)
	return &s, nil
}
