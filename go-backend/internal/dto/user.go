package dto

// CreateUserRequest is the request body for POST /api/users.
type CreateUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

// UpdateUserRequest is the request body for PUT /api/users/{id}.
type UpdateUserRequest struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// ToggleStatusRequest is the request body for PATCH /api/users/{id}/toggle-status.
type ToggleStatusRequest struct {
	IsActive bool `json:"is_active"`
}
