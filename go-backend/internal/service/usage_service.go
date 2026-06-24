// Package service provides UsageService for token usage recording and reporting.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// UsageService handles token usage recording and aggregation.
type UsageService struct {
	repo repository.UsageRepo
}

// NewUsageService creates a new UsageService.
func NewUsageService(repo repository.UsageRepo) *UsageService {
	return &UsageService{repo: repo}
}

// RecordUsage saves a token usage record asynchronously.
// This method is designed to never block or fail the main chat flow.
// All errors are logged but not returned.
func (s *UsageService) RecordUsage(ctx context.Context, u *model.TokenUsage) {
	// Compute total tokens if not set
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}

	// Cost estimation: if model price is not configured, set status to "unknown"
	// We never fabricate costs — requirement T8.3 boundary
	if u.CostStatus == "" {
		u.CostStatus = "unknown"
		u.EstimatedCost = 0
	}

	// Fire-and-forget: record in background goroutine to not block the chat flow
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.Create(bgCtx, u); err != nil {
			slog.Error("usage recording failed",
				"user_id", u.UserID,
				"session_id", u.SessionID,
				"error", err.Error(),
			)
		}
	}()
}

// GetSummary returns aggregated usage statistics for the given time range.
func (s *UsageService) GetSummary(ctx context.Context, from, to time.Time) (*model.UsageSummary, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -1) // default: last 24 hours
	}
	if to.IsZero() {
		to = time.Now().Add(1 * time.Second)
	}
	return s.repo.Summary(ctx, from, to)
}

// GetByRole returns per-role aggregated usage statistics for the given time range.
func (s *UsageService) GetByRole(ctx context.Context, from, to time.Time) ([]model.UsageByRole, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -1)
	}
	if to.IsZero() {
		to = time.Now().Add(1 * time.Second)
	}
	return s.repo.ByRole(ctx, from, to)
}

// GetTopUsers returns the top N users by token consumption.
func (s *UsageService) GetTopUsers(ctx context.Context, from, to time.Time, limit int) ([]model.TopUserUsage, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -1)
	}
	if to.IsZero() {
		to = time.Now().Add(1 * time.Second)
	}
	if limit <= 0 {
		limit = 10
	}
	return s.repo.TopUsers(ctx, from, to, limit)
}

// GetFullReport returns a comprehensive usage report including summary, per-role, and top users.
func (s *UsageService) GetFullReport(ctx context.Context, from, to time.Time) (map[string]any, error) {
	summary, err := s.GetSummary(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("service: usage summary: %w", err)
	}

	byRole, err := s.GetByRole(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("service: usage by role: %w", err)
	}

	topUsers, err := s.GetTopUsers(ctx, from, to, 10)
	if err != nil {
		return nil, fmt.Errorf("service: top users: %w", err)
	}

	return map[string]any{
		"summary":   summary,
		"by_role":   byRole,
		"top_users": topUsers,
		"from":      from.Format(time.RFC3339),
		"to":        to.Format(time.RFC3339),
	}, nil
}
