package model

import "time"

// SystemConfig represents a runtime business configuration parameter.
type SystemConfig struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Value       any       `json:"value"` // JSON (number, string, array, object)
	Category    string    `json:"category"`
	Description string    `json:"description"`
	UpdatedBy   *int64    `json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}
