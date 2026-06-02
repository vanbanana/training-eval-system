package model

import "time"

// LLMConfig represents an LLM provider configuration.
type LLMConfig struct {
	ID              int64     `json:"id"`
	Provider        string    `json:"provider"` // deepseek, tongyi, zhipu, moonshot
	BaseURL         string    `json:"base_url"`
	APIKeyEncrypted string    `json:"-"` // never expose in JSON
	ChatModel       string    `json:"chat_model"`
	EmbedModel      string    `json:"embed_model"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
