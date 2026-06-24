// Package repository provides the UsageRepo implementation for token usage tracking.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

// UsageRepo defines data access for token usage records.
type UsageRepo interface {
	Create(ctx context.Context, u *model.TokenUsage) error
	// Summary returns aggregated usage stats for the given time range.
	Summary(ctx context.Context, from, to time.Time) (*model.UsageSummary, error)
	// ByRole returns per-role aggregated usage stats for the given time range.
	ByRole(ctx context.Context, from, to time.Time) ([]model.UsageByRole, error)
	// TopUsers returns the top N users by total tokens in the given time range.
	TopUsers(ctx context.Context, from, to time.Time, limit int) ([]model.TopUserUsage, error)
}

// usageRepo implements UsageRepo backed by SQLite.
type usageRepo struct {
	db *store.DB
}

// NewUsageRepo creates a new UsageRepo.
func NewUsageRepo(db *store.DB) UsageRepo {
	return &usageRepo{db: db}
}

func (r *usageRepo) Create(ctx context.Context, u *model.TokenUsage) error {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO token_usage (user_id, user_role, agent_role, session_id, model, provider,
			prompt_tokens, completion_tokens, total_tokens, tool_call_count, success,
			latency_ms, cost_status, estimated_cost, error_code, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.UserID, u.UserRole, u.AgentRole, u.SessionID,
		u.Model, u.Provider,
		u.PromptTokens, u.CompletionTokens, u.TotalTokens,
		u.ToolCallCount, boolToInt(u.Success),
		u.LatencyMs, u.CostStatus, u.EstimatedCost, u.ErrorCode, now,
	)
	if err != nil {
		return fmt.Errorf("repo: create token usage: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repo: get usage id: %w", err)
	}
	u.ID = id
	u.CreatedAt = now
	return nil
}

func (r *usageRepo) Summary(ctx context.Context, from, to time.Time) (*model.UsageSummary, error) {
	fromStr := from.Format(time.RFC3339)
	toStr := to.Format(time.RFC3339)

	s := &model.UsageSummary{}
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(estimated_cost), 0),
			COALESCE(AVG(latency_ms), 0)
		FROM token_usage
		WHERE created_at >= ? AND created_at < ?`, fromStr, toStr,
	).Scan(&s.TotalRequests, &s.SuccessRequests, &s.FailedRequests,
		&s.TotalPromptTokens, &s.TotalCompletionTokens, &s.TotalTokens,
		&s.TotalEstimatedCost, &s.AvgLatencyMs)
	if err != nil {
		return nil, fmt.Errorf("repo: usage summary: %w", err)
	}

	if s.TotalRequests > 0 {
		s.FailureRate = float64(s.FailedRequests) / float64(s.TotalRequests)
	}

	// Determine cost status
	s.CostStatus = r.determineCostStatus(ctx, fromStr, toStr)

	return s, nil
}

func (r *usageRepo) ByRole(ctx context.Context, from, to time.Time) ([]model.UsageByRole, error) {
	fromStr := from.Format(time.RFC3339)
	toStr := to.Format(time.RFC3339)

	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT
			user_role,
			COUNT(*),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(estimated_cost), 0),
			COALESCE(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END), 0)
		FROM token_usage
		WHERE created_at >= ? AND created_at < ?
		GROUP BY user_role
		ORDER BY total_tokens DESC`, fromStr, toStr,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: usage by role: %w", err)
	}
	defer rows.Close()

	var results []model.UsageByRole
	for rows.Next() {
		var u model.UsageByRole
		var failedCount int64
		if err := rows.Scan(&u.Role, &u.TotalRequests, &u.TotalTokens,
			&u.PromptTokens, &u.CompletionTokens, &u.EstimatedCost, &failedCount); err != nil {
			return nil, fmt.Errorf("repo: scan usage by role: %w", err)
		}
		if u.TotalRequests > 0 {
			u.FailureRate = float64(failedCount) / float64(u.TotalRequests)
		}
		u.CostStatus = r.determineCostStatus(ctx, fromStr, toStr)
		results = append(results, u)
	}
	return results, rows.Err()
}

func (r *usageRepo) TopUsers(ctx context.Context, from, to time.Time, limit int) ([]model.TopUserUsage, error) {
	fromStr := from.Format(time.RFC3339)
	toStr := to.Format(time.RFC3339)

	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT tu.user_id, COALESCE(u.username, ''), tu.user_role,
			SUM(tu.total_tokens) as total_tok, COUNT(*) as total_req
		FROM token_usage tu
		LEFT JOIN users u ON tu.user_id = u.id
		WHERE tu.created_at >= ? AND tu.created_at < ?
		GROUP BY tu.user_id
		ORDER BY total_tok DESC
		LIMIT ?`, fromStr, toStr, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: top users: %w", err)
	}
	defer rows.Close()

	var results []model.TopUserUsage
	for rows.Next() {
		var u model.TopUserUsage
		if err := rows.Scan(&u.UserID, &u.Username, &u.Role, &u.TotalTokens, &u.TotalRequests); err != nil {
			return nil, fmt.Errorf("repo: scan top user: %w", err)
		}
		results = append(results, u)
	}
	return results, rows.Err()
}

// determineCostStatus checks whether all usage records in the range have cost data.
func (r *usageRepo) determineCostStatus(ctx context.Context, fromStr, toStr string) string {
	var total, withCost int
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN cost_status = 'calculated' THEN 1 ELSE 0 END), 0)
		FROM token_usage WHERE created_at >= ? AND created_at < ?`, fromStr, toStr,
	).Scan(&total, &withCost)
	if err != nil || total == 0 {
		return "unknown"
	}
	if withCost == total {
		return "calculated"
	}
	if withCost > 0 {
		return "partial"
	}
	return "unknown"
}
