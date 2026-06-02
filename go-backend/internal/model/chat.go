package model

import "time"

// ChatSession represents an AI chat session for a student.
type ChatSession struct {
	ID           int64     `json:"id"`
	StudentID    int64     `json:"student_id"`
	EvaluationID *int64    `json:"evaluation_id"`
	Title        string    `json:"title"`
	IsDeleted    bool      `json:"is_deleted"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
}

// ChatMessage represents a single message in a chat session.
type ChatMessage struct {
	ID               int64     `json:"id"`
	SessionID        int64     `json:"session_id"`
	Role             string    `json:"role"` // user, assistant, tool
	Content          string    `json:"content"`
	ToolCallID       *string   `json:"tool_call_id"`
	ToolName         *string   `json:"tool_name"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}
