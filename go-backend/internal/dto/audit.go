package dto

// AuditLogResponse is the response for a single audit log entry.
type AuditLogResponse struct {
	ID             int64  `json:"id"`
	OccurredAt     string `json:"occurred_at"`
	UserID         *int64 `json:"user_id"`
	Username       string `json:"username"`
	Role           string `json:"role"`
	Action         string `json:"action"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	Target         string `json:"target"`
	Result         string `json:"result"`
	Detail         string `json:"detail"`
	ClientIP       string `json:"client_ip"`
	TraceID        string `json:"trace_id"`
	SuspiciousFlag bool   `json:"suspicious_flag"`
	CreatedAt      string `json:"created_at"`
}
