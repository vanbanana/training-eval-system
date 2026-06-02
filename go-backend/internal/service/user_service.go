package service

import (
	"context"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// UserService handles user CRUD operations.
type UserService struct {
	repo repository.UserRepo
}

// NewUserService creates a new user service.
func NewUserService(repo repository.UserRepo) *UserService {
	return &UserService{repo: repo}
}

// GetByID retrieves a user by ID.
func (s *UserService) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves users with pagination and search.
func (s *UserService) List(ctx context.Context, params repository.ListParams) ([]model.User, int64, error) {
	return s.repo.List(ctx, params)
}

// Create creates a new user with hashed password.
func (s *UserService) Create(ctx context.Context, u *model.User, password string) error {
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("user_service: hash password: %w", err)
	}
	u.PasswordHash = hash
	u.IsActive = true
	return s.repo.Create(ctx, u)
}

// Update updates user display_name and role.
func (s *UserService) Update(ctx context.Context, u *model.User) error {
	return s.repo.Update(ctx, u)
}

// Delete removes a user.
func (s *UserService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ToggleActive enables or disables a user account.
func (s *UserService) ToggleActive(ctx context.Context, id int64, active bool) error {
	return s.repo.ToggleActive(ctx, id, active)
}

// ResetPassword sets a new password for a user.
func (s *UserService) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	// We need a direct update for password — use a dedicated method or update the full user
	return s.repo.Update(ctx, user)
}

// GetDisplayNames returns a map of user ID → display_name for the given IDs.
func (s *UserService) GetDisplayNames(ctx context.Context, ids map[int64]struct{}) map[int64]string {
	result := make(map[int64]string, len(ids))
	for id := range ids {
		user, err := s.repo.GetByID(ctx, id)
		if err == nil {
			result[id] = user.DisplayName
		}
	}
	return result
}

// GetUsersMap returns a map of user ID → User for the given IDs.
func (s *UserService) GetUsersMap(ctx context.Context, ids map[int64]struct{}) map[int64]*model.User {
	result := make(map[int64]*model.User, len(ids))
	for id := range ids {
		user, err := s.repo.GetByID(ctx, id)
		if err == nil {
			result[id] = user
		}
	}
	return result
}
