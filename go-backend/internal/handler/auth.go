package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	svc *service.AuthService
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		Error(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	result, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	JSON(w, http.StatusOK, dto.LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		User:         userToResponse(result.User),
	})
}

// Refresh handles POST /api/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	JSON(w, http.StatusOK, dto.RefreshResponse{
		AccessToken: token,
		TokenType:   "bearer",
	})
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := h.svc.GetMe(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusNotFound, "User not found")
		return
	}

	JSON(w, http.StatusOK, userToResponse(user))
}

// Logout handles POST /api/auth/logout (stateless — just returns success).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Logged out"})
}
