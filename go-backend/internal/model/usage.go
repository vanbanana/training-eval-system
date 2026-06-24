package model

// TokenUsage records per-request LLM token consumption and cost data.
// NOTE: Never stores API keys or full prompts (security requirement T8.3).
type TokenUsage struct {
	ID               int64   `json:"id"`
	UserID           int64   `json:"user_id"`
	UserRole         string  `json:"user_role"`
	AgentRole        string  `json:"agent_role"`
	SessionID        int64   `json:"session_id"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	ToolCallCount    int     `json:"tool_call_count"`
	Success          bool    `json:"success"`
	LatencyMs        int64   `json:"latency_ms"`
	CostStatus       string  `json:"cost_status"` // "calculated" | "unknown"
	EstimatedCost    float64 `json:"estimated_cost"`
	ErrorCode        string  `json:"error_code,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

// UsageSummary holds aggregated usage statistics for a time range.
type UsageSummary struct {
	TotalRequests         int64   `json:"total_requests"`
	SuccessRequests       int64   `json:"success_requests"`
	FailedRequests        int64   `json:"failed_requests"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64   `json:"total_completion_tokens"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalEstimatedCost    float64 `json:"total_estimated_cost"`
	CostStatus            string  `json:"cost_status"` // "calculated" | "unknown" | "partial"
	AvgLatencyMs          float64 `json:"avg_latency_ms"`
	FailureRate           float64 `json:"failure_rate"`
}

// UsageByRole holds per-role aggregated usage statistics.
type UsageByRole struct {
	Role             string  `json:"role"`
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
	CostStatus       string  `json:"cost_status"`
	FailureRate      float64 `json:"failure_rate"`
}

// TopUserUsage holds per-user high-usage information.
type TopUserUsage struct {
	UserID        int64  `json:"user_id"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	TotalTokens   int64  `json:"total_tokens"`
	TotalRequests int64  `json:"total_requests"`
}
