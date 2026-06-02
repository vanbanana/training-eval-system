package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteLLMConfigRepo struct{ db *store.DB }

func NewLLMConfigRepo(db *store.DB) LLMConfigRepo { return &SQLiteLLMConfigRepo{db: db} }

func (r *SQLiteLLMConfigRepo) GetByID(ctx context.Context, id int64) (*model.LLMConfig, error) {
	var c model.LLMConfig
	var isActive int
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,provider,base_url,api_key_encrypted,chat_model,embed_model,is_active,created_at,updated_at FROM llm_configs WHERE id=?", id).Scan(
		&c.ID, &c.Provider, &c.BaseURL, &c.APIKeyEncrypted, &c.ChatModel, &c.EmbedModel, &isActive, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("llm_config_repo: not found")
		}
		return nil, err
	}
	c.IsActive = isActive != 0
	c.CreatedAt = parseTime(createdAt.String)
	c.UpdatedAt = parseTime(updatedAt.String)
	return &c, nil
}

func (r *SQLiteLLMConfigRepo) GetActive(ctx context.Context) (*model.LLMConfig, error) {
	var c model.LLMConfig
	var isActive int
	var createdAt, updatedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT id,provider,base_url,api_key_encrypted,chat_model,embed_model,is_active,created_at,updated_at FROM llm_configs WHERE is_active=1 LIMIT 1").Scan(
		&c.ID, &c.Provider, &c.BaseURL, &c.APIKeyEncrypted, &c.ChatModel, &c.EmbedModel, &isActive, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("llm_config_repo: no active config")
		}
		return nil, err
	}
	c.IsActive = isActive != 0
	c.CreatedAt = parseTime(createdAt.String)
	c.UpdatedAt = parseTime(updatedAt.String)
	return &c, nil
}

func (r *SQLiteLLMConfigRepo) List(ctx context.Context) ([]model.LLMConfig, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		"SELECT id,provider,base_url,api_key_encrypted,chat_model,embed_model,is_active,created_at,updated_at FROM llm_configs ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []model.LLMConfig
	for rows.Next() {
		var c model.LLMConfig
		var isActive int
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Provider, &c.BaseURL, &c.APIKeyEncrypted, &c.ChatModel, &c.EmbedModel, &isActive, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.IsActive = isActive != 0
		c.CreatedAt = parseTime(createdAt.String)
		c.UpdatedAt = parseTime(updatedAt.String)
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *SQLiteLLMConfigRepo) Create(ctx context.Context, c *model.LLMConfig) error {
	res, err := r.db.Writer.ExecContext(ctx,
		"INSERT INTO llm_configs (provider,base_url,api_key_encrypted,chat_model,embed_model,is_active) VALUES (?,?,?,?,?,?)",
		c.Provider, c.BaseURL, c.APIKeyEncrypted, c.ChatModel, c.EmbedModel, boolToInt(c.IsActive))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return nil
}

func (r *SQLiteLLMConfigRepo) Update(ctx context.Context, c *model.LLMConfig) error {
	_, err := r.db.Writer.ExecContext(ctx,
		"UPDATE llm_configs SET provider=?,base_url=?,api_key_encrypted=?,chat_model=?,embed_model=?,is_active=?,updated_at=datetime('now') WHERE id=?",
		c.Provider, c.BaseURL, c.APIKeyEncrypted, c.ChatModel, c.EmbedModel, boolToInt(c.IsActive), c.ID)
	return err
}

func (r *SQLiteLLMConfigRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM llm_configs WHERE id=?", id)
	return err
}
