package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteAuditRepo struct {
	db *store.DB
}

func NewAuditRepo(db *store.DB) AuditRepo {
	return &SQLiteAuditRepo{db: db}
}

func (r *SQLiteAuditRepo) Create(ctx context.Context, log *model.AuditLog) error {
	payloadJSON, _ := json.Marshal(log.Payload)
	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO audit_logs (user_id, username, role, action, target_type, target_id, target, result, detail, payload, client_ip, user_agent, trace_id, suspicious_flag, ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.UserID, log.Username, log.Role, log.Action, log.TargetType, log.TargetID,
		log.Target, log.Result, log.Detail, string(payloadJSON), log.ClientIP,
		log.UserAgent, log.TraceID, boolToInt(log.SuspiciousFlag), log.IP)
	return err
}

func (r *SQLiteAuditRepo) List(ctx context.Context, params ListParams, userID *int64, action *string) ([]model.AuditLog, int64, error) {
	where := "1=1"
	args := []any{}

	if userID != nil {
		where += " AND user_id=?"
		args = append(args, *userID)
	}
	if action != nil {
		where += " AND action=?"
		args = append(args, *action)
	}
	if params.Search != "" {
		where += " AND (action LIKE ? OR target LIKE ? OR detail LIKE ?)"
		like := "%" + params.Search + "%"
		args = append(args, like, like, like)
	}

	var total int64
	if err := r.db.Reader.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	querySQL := fmt.Sprintf(
		`SELECT id, occurred_at, user_id, username, role, action, target_type, target_id, target,
		        result, detail, payload, client_ip, user_agent, trace_id, suspicious_flag, ip, created_at
		 FROM audit_logs WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, params.PageSize, params.Offset())

	rows, err := r.db.Reader.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var l model.AuditLog
		var occurredAt, createdAt sql.NullString
		var payloadStr sql.NullString
		var suspFlag int
		if err := rows.Scan(&l.ID, &occurredAt, &l.UserID, &l.Username, &l.Role, &l.Action,
			&l.TargetType, &l.TargetID, &l.Target, &l.Result, &l.Detail, &payloadStr,
			&l.ClientIP, &l.UserAgent, &l.TraceID, &suspFlag, &l.IP, &createdAt); err != nil {
			return nil, 0, err
		}
		l.OccurredAt = parseTime(occurredAt.String)
		l.CreatedAt = parseTime(createdAt.String)
		l.SuspiciousFlag = suspFlag != 0
		if payloadStr.Valid {
			_ = json.Unmarshal([]byte(payloadStr.String), &l.Payload)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}
