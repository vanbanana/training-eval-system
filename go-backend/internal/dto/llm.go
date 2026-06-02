package dto

// CreateLLMConfigRequest is the request for POST /api/llm/configs.
type CreateLLMConfigRequest struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	ChatModel  string `json:"chat_model"`
	EmbedModel string `json:"embed_model"`
}

// LLMConfigResponse is the response for a single LLM config (API key redacted).
type LLMConfigResponse struct {
	ID         int64  `json:"id"`
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	ChatModel  string `json:"chat_model"`
	EmbedModel string `json:"embed_model"`
	IsActive   bool   `json:"is_active"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// LLMTestResponse is the response for POST /api/llm/test.
type LLMTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency_ms"`
}
