package dto

// AgentSessionResponse is the response for a single agent session.
type AgentSessionResponse struct {
	ID           int64  `json:"id"`
	OwnerID      int64  `json:"owner_id"`
	OwnerRole    string `json:"owner_role"`
	AgentRole    string `json:"agent_role"`
	Title        string `json:"title"`
	ContextJSON  string `json:"context_json"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
}

// CreateAgentSessionRequest is the request for POST /api/agent/sessions.
type CreateAgentSessionRequest struct {
	Title     string           `json:"title"`
	AgentRole string           `json:"agent_role"`
	Context   *AgentContextReq `json:"context,omitempty"`
}

// AgentContextReq is the optional context payload for agent sessions.
type AgentContextReq struct {
	EvaluationID *int64 `json:"evaluation_id,omitempty"`
	TaskID       *int64 `json:"task_id,omitempty"`
	ClassID      *int64 `json:"class_id,omitempty"`
	CourseID     *int64 `json:"course_id,omitempty"`
}

// AgentMessageResponse is the response for a single agent message.
type AgentMessageResponse struct {
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

// AgentStreamRequest is the request for POST /api/agent/stream.
type AgentStreamRequest struct {
	SessionID          int64            `json:"session_id"`
	Message            string           `json:"message"`
	AgentRole          string           `json:"agent_role"`
	Context            *AgentContextReq `json:"context,omitempty"`
	ForceContextSwitch bool             `json:"force_context_switch,omitempty"`
}

// AgentErrorResponse is the unified error format for agent API.
type AgentErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Unified agent error codes.
const (
	AgentErrAuthRequired      = "AGENT_AUTH_REQUIRED"
	AgentErrRoleMismatch      = "AGENT_ROLE_MISMATCH"
	AgentErrSessionNotFound   = "AGENT_SESSION_NOT_FOUND"
	AgentErrSessionForbidden  = "AGENT_SESSION_FORBIDDEN"
	AgentErrContextNotFound   = "AGENT_CONTEXT_NOT_FOUND"
	AgentErrContextForbidden  = "AGENT_CONTEXT_FORBIDDEN"
	AgentErrInvalidRequest    = "AGENT_INVALID_REQUEST"
	AgentErrMessageTooLong    = "AGENT_MESSAGE_TOO_LONG"
	AgentErrDailyLimit        = "AGENT_DAILY_LIMIT"
	AgentErrSessionLimit      = "AGENT_SESSION_LIMIT"
	AgentErrLLMNotConfigured  = "AGENT_LLM_NOT_CONFIGURED"
	AgentErrToolFailed        = "AGENT_TOOL_FAILED"
	AgentErrStreamInterrupted = "AGENT_STREAM_INTERRUPTED"
	AgentErrInternal          = "AGENT_INTERNAL"
	AgentErrContextSwitch     = "AGENT_CONTEXT_SWITCH_REQUIRED"
	AgentErrCrossRoleContext  = "AGENT_CROSS_ROLE_CONTEXT"
	AgentErrSensitiveWord     = "AGENT_SENSITIVE_CONTENT"
	AgentErrConcurrentLimit   = "AGENT_CONCURRENT_LIMIT"
	AgentErrLLMTimeout        = "AGENT_LLM_TIMEOUT"
	AgentErrLLMUnavailable    = "AGENT_LLM_UNAVAILABLE"
)
