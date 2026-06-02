package model

import "time"

// AuditLog represents an append-only audit trail entry.
type AuditLog struct {
	ID             int64     `json:"id"`
	OccurredAt     time.Time `json:"occurred_at"`
	UserID         *int64    `json:"user_id"`
	Username       string    `json:"username"`
	Role           string    `json:"role"`
	Action         string    `json:"action"`
	TargetType     string    `json:"target_type"`
	TargetID       string    `json:"target_id"`
	Target         string    `json:"target"`
	Result         string    `json:"result"` // success, failure
	Detail         string    `json:"detail"`
	Payload        any       `json:"payload"` // JSON
	ClientIP       string    `json:"client_ip"`
	UserAgent      string    `json:"user_agent"`
	TraceID        string    `json:"trace_id"`
	SuspiciousFlag bool      `json:"suspicious_flag"`
	IP             string    `json:"ip"`
	CreatedAt      time.Time `json:"created_at"`
}
