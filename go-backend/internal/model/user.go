// Package model defines domain model structs.
package model

import "time"

// User represents a system user (admin, teacher, or student).
type User struct {
	ID               int64      `json:"id"`
	Username         string     `json:"username"`
	DisplayName      string     `json:"display_name"`
	PasswordHash     string     `json:"-"`
	Role             string     `json:"role"` // admin, teacher, student
	IsActive         bool       `json:"is_active"`
	FailedLoginCount int        `json:"failed_login_count"`
	LockedUntil      *time.Time `json:"locked_until"`
	LastLoginAt      *time.Time `json:"last_login_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
