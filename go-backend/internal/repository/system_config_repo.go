package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteSystemConfigRepo struct{ db *store.DB }

func NewSystemConfigRepo(db *store.DB) SystemConfigRepo {
	return &SQLiteSystemConfigRepo{db: db}
}

func (r *SQLiteSystemConfigRepo) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var c model.SystemConfig
	var valueStr string
	var updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,key,value,category,description,updated_by,updated_at FROM system_config WHERE key=?", key).Scan(
		&c.ID, &c.Key, &valueStr, &c.Category, &c.Description, &c.UpdatedBy, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("system_config_repo: key %q not found", key)
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(valueStr), &c.Value)
	c.UpdatedAt = parseTime(updatedAt.String)
	return &c, nil
}

func (r *SQLiteSystemConfigRepo) List(ctx context.Context, category *string) ([]model.SystemConfig, error) {
	where := "1=1"
	args := []any{}
	if category != nil {
		where += " AND category=?"
		args = append(args, *category)
	}
	rows, err := r.db.Reader.QueryContext(ctx,
		fmt.Sprintf("SELECT id,key,value,category,description,updated_by,updated_at FROM system_config WHERE %s ORDER BY key", where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []model.SystemConfig
	for rows.Next() {
		var c model.SystemConfig
		var valueStr string
		var updatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Key, &valueStr, &c.Category, &c.Description, &c.UpdatedBy, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(valueStr), &c.Value)
		c.UpdatedAt = parseTime(updatedAt.String)
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *SQLiteSystemConfigRepo) Upsert(ctx context.Context, cfg *model.SystemConfig) error {
	valueJSON, _ := json.Marshal(cfg.Value)
	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO system_config (key,value,category,description,updated_by,updated_at)
		 VALUES (?,?,?,?,?,datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, category=excluded.category,
		   description=excluded.description, updated_by=excluded.updated_by, updated_at=excluded.updated_at`,
		cfg.Key, string(valueJSON), cfg.Category, cfg.Description, cfg.UpdatedBy)
	return err
}
