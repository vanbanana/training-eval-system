package dto

// UpdateProfileRequest is the request for PATCH /api/account/profile.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

// ChangePasswordRequest is the request for POST /api/account/change-password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ResetPasswordRequest is the request for POST /api/users/{id}/reset-password.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}
