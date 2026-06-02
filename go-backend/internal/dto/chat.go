package dto

// ChatSessionResponse is the response for a single chat session.
type ChatSessionResponse struct {
	ID           int64  `json:"id"`
	StudentID    int64  `json:"student_id"`
	EvaluationID *int64 `json:"evaluation_id"`
	Title        string `json:"title"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
}

// CreateSessionRequest is the request for POST /api/chat/sessions.
type CreateSessionRequest struct {
	Title        string `json:"title"`
	EvaluationID *int64 `json:"evaluation_id"`
}

// ChatMessageResponse is the response for a single chat message.
type ChatMessageResponse struct {
	ID               int64   `json:"id"`
	SessionID        int64   `json:"session_id"`
	Role             string  `json:"role"`
	Content          string  `json:"content"`
	ToolCallID       *string `json:"tool_call_id"`
	ToolName         *string `json:"tool_name"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CreatedAt        string  `json:"created_at"`
}

// ChatStreamRequest is the request for POST /api/chat/stream.
type ChatStreamRequest struct {
	SessionID    int64  `json:"session_id"`
	Message      string `json:"message"`
	EvaluationID *int64 `json:"evaluation_id"`
}
