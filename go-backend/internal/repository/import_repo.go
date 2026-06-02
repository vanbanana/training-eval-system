package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteImportRepo struct{ db *store.DB }

func NewImportRepo(db *store.DB) ImportRepo { return &SQLiteImportRepo{db: db} }

func (r *SQLiteImportRepo) GetByID(ctx context.Context, id int64) (*model.ImportJob, error) {
	var j model.ImportJob
	var createdAt, completedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id,operator_id,job_type,status,total_count,success_count,failed_count,failed_file_path,created_at,completed_at
		 FROM import_jobs WHERE id=?`, id).Scan(
		&j.ID, &j.OperatorID, &j.JobType, &j.Status, &j.TotalCount, &j.SuccessCount,
		&j.FailedCount, &j.FailedFilePath, &createdAt, &completedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("import_repo: not found")
		}
		return nil, err
	}
	j.CreatedAt = parseTime(createdAt.String)
	j.CompletedAt = parseNullTime(completedAt)
	return &j, nil
}

func (r *SQLiteImportRepo) List(ctx context.Context, operatorID *int64, params ListParams) ([]model.ImportJob, int64, error) {
	where := "1=1"
	args := []any{}
	if operatorID != nil {
		where += " AND operator_id=?"
		args = append(args, *operatorID)
	}
	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM import_jobs WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	querySQL := fmt.Sprintf(
		`SELECT id,operator_id,job_type,status,total_count,success_count,failed_count,failed_file_path,created_at,completed_at
		 FROM import_jobs WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, params.PageSize, params.Offset())
	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var jobs []model.ImportJob
	for rows.Next() {
		var j model.ImportJob
		var createdAt, completedAt sql.NullString
		if err := rows.Scan(&j.ID, &j.OperatorID, &j.JobType, &j.Status, &j.TotalCount, &j.SuccessCount,
			&j.FailedCount, &j.FailedFilePath, &createdAt, &completedAt); err != nil {
			return nil, 0, err
		}
		j.CreatedAt = parseTime(createdAt.String)
		j.CompletedAt = parseNullTime(completedAt)
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

func (r *SQLiteImportRepo) Create(ctx context.Context, j *model.ImportJob) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO import_jobs (operator_id,job_type,status,total_count,success_count,failed_count) VALUES (?,?,?,?,?,?)",
		j.OperatorID, j.JobType, j.Status, j.TotalCount, j.SuccessCount, j.FailedCount)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	j.ID = id
	return nil
}

func (r *SQLiteImportRepo) Update(ctx context.Context, j *model.ImportJob) error {
	var completedStr *string
	if j.CompletedAt != nil {
		s := j.CompletedAt.Format("2006-01-02 15:04:05")
		completedStr = &s
	}
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE import_jobs SET status=?,total_count=?,success_count=?,failed_count=?,failed_file_path=?,completed_at=? WHERE id=?",
		j.Status, j.TotalCount, j.SuccessCount, j.FailedCount, j.FailedFilePath, completedStr, j.ID)
	return err
}

func (r *SQLiteImportRepo) CreateRecord(ctx context.Context, rec *model.ImportRecord) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO import_records (job_id,row_number,status,error_message) VALUES (?,?,?,?)",
		rec.JobID, rec.RowNumber, rec.Status, rec.ErrorMessage)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	return nil
}
