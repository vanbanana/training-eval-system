package model

import "time"

// Notification represents a user notification.
type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Payload   any       `json:"payload"` // JSON
	IsRead    bool      `json:"is_read"`
	Link      string    `json:"link"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationPref represents a user's notification preference for an event type.
type NotificationPref struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
}
