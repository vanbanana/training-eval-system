// Package service implements business logic orchestration.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// AuthService handles authentication operations.
type AuthService struct {
	userRepo   repository.UserRepo
	auditRepo  repository.AuditRepo
	lockout    *middleware.AccountLockout
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService creates a new auth service.
func NewAuthService(
	userRepo repository.UserRepo,
	auditRepo repository.AuditRepo,
	lockout *middleware.AccountLockout,
	jwtSecret string,
	accessTTL, refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		auditRepo:  auditRepo,
		lockout:    lockout,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// LoginResult holds the tokens returned after successful login.
type LoginResult struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	User         *model.User `json:"user"`
}

// Login authenticates a user and returns JWT tokens.
func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	// Check lockout
	if s.lockout.IsLocked(username) {
		return nil, fmt.Errorf("auth: account locked due to too many failed attempts")
	}

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		s.lockout.RecordFailure(username)
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("auth: account disabled")
	}

	// Check if locked in DB
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("auth: account locked until %s", user.LockedUntil.Format(time.RFC3339))
	}

	// Verify password
	if err := crypto.VerifyPassword(user.PasswordHash, password); err != nil {
		s.lockout.RecordFailure(username)
		user.FailedLoginCount++
		if user.FailedLoginCount >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute)
			_ = s.userRepo.UpdateLoginState(ctx, user.ID, user.FailedLoginCount, &lockUntil)
		} else {
			_ = s.userRepo.UpdateLoginState(ctx, user.ID, user.FailedLoginCount, nil)
		}
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	// Success — reset lockout
	s.lockout.RecordSuccess(username)
	_ = s.userRepo.UpdateLoginState(ctx, user.ID, 0, nil)
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Generate tokens
	now := time.Now()
	accessClaims := &crypto.Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
		Iat:      now.Unix(),
		Exp:      now.Add(s.accessTTL).Unix(),
	}
	accessToken, err := crypto.SignToken(s.jwtSecret, accessClaims)
	if err != nil {
		return nil, fmt.Errorf("auth: sign access token: %w", err)
	}

	refreshClaims := &crypto.Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "refresh",
		Iat:      now.Unix(),
		Exp:      now.Add(s.refreshTTL).Unix(),
	}
	refreshToken, err := crypto.SignToken(s.jwtSecret, refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("auth: sign refresh token: %w", err)
	}

	// Audit log
	_ = s.auditRepo.Create(ctx, &model.AuditLog{
		UserID:   &user.ID,
		Username: user.Username,
		Role:     user.Role,
		Action:   "login",
		Result:   "success",
	})

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "bearer",
		User:         user,
	}, nil
}

// Refresh generates a new access token from a valid refresh token.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := crypto.VerifyToken(s.jwtSecret, refreshToken)
	if err != nil {
		return "", fmt.Errorf("auth: invalid refresh token: %w", err)
	}
	if claims.Type != "refresh" {
		return "", fmt.Errorf("auth: not a refresh token")
	}

	now := time.Now()
	newClaims := &crypto.Claims{
		Sub:      claims.Sub,
		Username: claims.Username,
		Role:     claims.Role,
		Type:     "access",
		Iat:      now.Unix(),
		Exp:      now.Add(s.accessTTL).Unix(),
	}
	return crypto.SignToken(s.jwtSecret, newClaims)
}

// GetMe returns the current user from claims.
func (s *AuthService) GetMe(ctx context.Context, userID int64) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
