package dto

// NotificationResponse is the response for a single notification.
type NotificationResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Payload   any    `json:"payload"`
	IsRead    bool   `json:"is_read"`
	Link      string `json:"link"`
	CreatedAt string `json:"created_at"`
}

// NotificationListResponse wraps notifications with unread count.
type NotificationListResponse struct {
	Items       []NotificationResponse `json:"items"`
	UnreadCount int64                  `json:"unread_count"`
}

// UpdatePreferencesRequest is the request for PUT /api/notifications/preferences.
type UpdatePreferencesRequest struct {
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
}

// PreferencesResponse is the response for GET /api/notifications/preferences.
type PreferencesResponse map[string]bool
