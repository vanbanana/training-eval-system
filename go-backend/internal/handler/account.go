package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/service"
)

type AccountHandler struct{ svc *service.UserService }

func NewAccountHandler(svc *service.UserService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	user, err := h.svc.GetByID(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}
	JSON(w, http.StatusOK, userToResponse(user))
}

func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProfileRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	user, err := h.svc.GetByID(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}
	user.DisplayName = req.DisplayName
	if err := h.svc.Update(r.Context(), user); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Profile updated"})
}

func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ChangePasswordRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	user, err := h.svc.GetByID(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}
	// Verify old password
	if err := crypto.VerifyPassword(user.PasswordHash, req.OldPassword); err != nil {
		Error(w, http.StatusBadRequest, "Incorrect old password")
		return
	}
	if err := h.svc.ResetPassword(r.Context(), claims.Sub, req.NewPassword); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Password changed"})
}
