package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteUploadRepo struct {
	db *store.DB
}

func NewUploadRepo(db *store.DB) UploadRepo {
	return &SQLiteUploadRepo{db: db}
}

func (r *SQLiteUploadRepo) GetByID(ctx context.Context, id int64) (*model.Upload, error) {
	var u model.Upload
	var isDeleted int
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, task_id, student_id, filename, file_type, file_size, storage_path,
		        sha256, parse_status, version, is_deleted, created_at, updated_at
		 FROM uploads WHERE id=?`, id).Scan(
		&u.ID, &u.TaskID, &u.StudentID, &u.Filename, &u.FileType, &u.FileSize,
		&u.StoragePath, &u.SHA256, &u.ParseStatus, &u.Version, &isDeleted, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("upload_repo: not found")
		}
		return nil, fmt.Errorf("upload_repo: get: %w", err)
	}
	u.IsDeleted = isDeleted != 0
	u.CreatedAt = parseTime(createdAt.String)
	u.UpdatedAt = parseTime(updatedAt.String)
	return &u, nil
}

func (r *SQLiteUploadRepo) List(ctx context.Context, params UploadListParams) ([]model.Upload, int64, error) {
	where := "is_deleted=0"
	args := []any{}

	if params.TaskID != nil {
		where += " AND task_id=?"
		args = append(args, *params.TaskID)
	}
	if params.StudentID != nil {
		where += " AND student_id=?"
		args = append(args, *params.StudentID)
	}
	if params.ParseStatus != nil {
		where += " AND parse_status=?"
		args = append(args, *params.ParseStatus)
	}

	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM uploads WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	querySQL := fmt.Sprintf(
		`SELECT id, task_id, student_id, filename, file_type, file_size, storage_path,
		        sha256, parse_status, version, is_deleted, created_at, updated_at
		 FROM uploads WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, params.PageSize, params.Offset())

	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var uploads []model.Upload
	for rows.Next() {
		var u model.Upload
		var isDeleted int
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&u.ID, &u.TaskID, &u.StudentID, &u.Filename, &u.FileType, &u.FileSize,
			&u.StoragePath, &u.SHA256, &u.ParseStatus, &u.Version, &isDeleted, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		u.IsDeleted = isDeleted != 0
		u.CreatedAt = parseTime(createdAt.String)
		u.UpdatedAt = parseTime(updatedAt.String)
		uploads = append(uploads, u)
	}
	return uploads, total, rows.Err()
}

func (r *SQLiteUploadRepo) Create(ctx context.Context, u *model.Upload) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		u.TaskID, u.StudentID, u.Filename, u.FileType, u.FileSize, u.StoragePath, u.SHA256, u.ParseStatus, u.Version)
	if err != nil {
		return fmt.Errorf("upload_repo: create: %w", err)
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (r *SQLiteUploadRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE uploads SET parse_status=?, updated_at=datetime('now') WHERE id=?", status, id)
	return err
}

func (r *SQLiteUploadRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE uploads SET is_deleted=1, updated_at=datetime('now') WHERE id=?", id)
	return err
}

func (r *SQLiteUploadRepo) GetParseResult(ctx context.Context, uploadID int64) (*model.ParseResult, error) {
	var pr model.ParseResult
	var structured, embedding sql.NullString
	var parsedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, upload_id, structured_content, raw_text, simhash, embedding, error_message, parsed_at
		 FROM parse_results WHERE upload_id=?`, uploadID).Scan(
		&pr.ID, &pr.UploadID, &structured, &pr.RawText, &pr.SimHash, &embedding, &pr.ErrorMessage, &parsedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if structured.Valid {
		_ = json.Unmarshal([]byte(structured.String), &pr.StructuredContent)
	}
	if embedding.Valid {
		_ = json.Unmarshal([]byte(embedding.String), &pr.Embedding)
	}
	pr.ParsedAt = parseTime(parsedAt.String)
	return &pr, nil
}

func (r *SQLiteUploadRepo) SaveParseResult(ctx context.Context, pr *model.ParseResult) error {
	structuredJSON, _ := json.Marshal(pr.StructuredContent)
	embeddingJSON, _ := json.Marshal(pr.Embedding)

	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT OR REPLACE INTO parse_results (upload_id, structured_content, raw_text, simhash, embedding, error_message, parsed_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		pr.UploadID, string(structuredJSON), pr.RawText, pr.SimHash, string(embeddingJSON), pr.ErrorMessage)
	if err != nil {
		return fmt.Errorf("upload_repo: save parse result: %w", err)
	}
	id, _ := res.LastInsertId()
	pr.ID = id
	return nil
}

func (r *SQLiteUploadRepo) SaveVerifyResult(ctx context.Context, vr *model.VerifyResult) error {
	checkpointsJSON, _ := json.Marshal(vr.Checkpoints)
	missingJSON, _ := json.Marshal(vr.MissingItems)
	logicJSON, _ := json.Marshal(vr.LogicIssues)

	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT OR REPLACE INTO verify_results (upload_id, match_rate, checkpoints, missing_items, logic_issues, overall_confidence, verified_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		vr.UploadID, vr.MatchRate, string(checkpointsJSON), string(missingJSON), string(logicJSON), vr.OverallConfidence)
	if err != nil {
		return fmt.Errorf("upload_repo: save verify result: %w", err)
	}
	id, _ := res.LastInsertId()
	vr.ID = id
	return nil
}
