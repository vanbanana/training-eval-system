package model

// AgentSession represents an AI agent conversation session.
type AgentSession struct {
	ID           int64  `json:"id"`
	OwnerID      int64  `json:"owner_id"`
	OwnerRole    string `json:"owner_role"`
	AgentRole    string `json:"agent_role"`
	Title        string `json:"title"`
	ContextJSON  string `json:"context_json"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
}

// AgentMessage represents a single message within an agent session.
type AgentMessage struct {
	ID               int64   `json:"id"`
	SessionID        int64   `json:"session_id"`
	Role             string  `json:"role"`
	Content          string  `json:"content"`
	ToolCallID       *string `json:"tool_call_id,omitempty"`
	ToolName         *string `json:"tool_name,omitempty"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CreatedAt        string  `json:"created_at"`
}
