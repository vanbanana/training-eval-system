package handler

import (
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

// UsersHandler handles user management endpoints.
type UsersHandler struct {
	svc *service.UserService
}

func NewUsersHandler(svc *service.UserService) *UsersHandler {
	return &UsersHandler{svc: svc}
}

// List handles GET /api/users.
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.ListParams{
		Page:     QueryInt(r, "page", 1),
		PageSize: QueryInt(r, "page_size", 200),
		Search:   QueryStr(r, "search", ""),
		SortBy:   QueryStr(r, "sort_by", ""),
		SortDir:  QueryStr(r, "sort_dir", "asc"),
	}

	users, _, err := h.svc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		items = append(items, userToResponse(&u))
	}

	JSON(w, http.StatusOK, items)
}

// Get handles GET /api/users/{id}.
func (h *UsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}
	JSON(w, http.StatusOK, userToResponse(user))
}

// Create handles POST /api/users.
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user := &model.User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Role:        req.Role,
	}

	if err := h.svc.Create(r.Context(), user, req.Password); err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}

	JSON(w, http.StatusCreated, userToResponse(user))
}

// Update handles PUT /api/users/{id}.
func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req dto.UpdateUserRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}

	user.DisplayName = req.DisplayName
	user.Role = req.Role

	if err := h.svc.Update(r.Context(), user); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	JSON(w, http.StatusOK, userToResponse(user))
}

// Delete handles DELETE /api/users/{id}.
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "User deleted"})
}

// ToggleStatus handles PATCH /api/users/{id}/toggle-status.
func (h *UsersHandler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}
	newActive := !user.IsActive
	if err := h.svc.ToggleActive(r.Context(), id, newActive); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	user.IsActive = newActive
	JSON(w, http.StatusOK, userToResponse(user))
}

// ResetPassword handles POST /api/users/{id}/reset-password.
func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	var req dto.ResetPasswordRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id, req.NewPassword); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Password reset"})
}

// --- helper ---

func userToResponse(u *model.User) dto.UserResponse {
	resp := dto.UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.Format(time.RFC3339),
	}
	if u.LastLoginAt != nil {
		s := u.LastLoginAt.Format(time.RFC3339)
		resp.LastLoginAt = &s
	}
	return resp
}
