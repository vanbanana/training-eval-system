// Package service — AgentService provides business logic for the unified AI agent API.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// Limits for agent messages and sessions.
const (
	maxAgentMessageLength = 500
)

// RoleQuota defines per-role message limits.
type RoleQuota struct {
	SessionLimit int
	DailyLimit   int
}

// Default role-based quotas (safe defaults when config is not provided).
var defaultQuotas = map[string]RoleQuota{
	"student": {SessionLimit: 20, DailyLimit: 50},
	"teacher": {SessionLimit: 50, DailyLimit: 200},
	"admin":   {SessionLimit: 50, DailyLimit: 300},
}

// sensitiveAgentKeywords are basic content-filter words.
var sensitiveAgentKeywords = []string{
	"hack", "exploit", "inject", "绕过", "入侵", "攻击",
}

// AgentService wraps AgentRepo with validation and quota enforcement.
type AgentService struct {
	repo   repository.AgentRepo
	quotas map[string]RoleQuota
}

// NewAgentService creates a new AgentService with default quotas.
func NewAgentService(repo repository.AgentRepo) *AgentService {
	return &AgentService{repo: repo, quotas: defaultQuotas}
}

// NewAgentServiceWithQuotas creates an AgentService with custom role-based quotas.
func NewAgentServiceWithQuotas(repo repository.AgentRepo, quotas map[string]RoleQuota) *AgentService {
	if quotas == nil {
		quotas = defaultQuotas
	}
	return &AgentService{repo: repo, quotas: quotas}
}

// GetQuota returns the quota for a given user role. Returns safe defaults for unknown roles.
func (s *AgentService) GetQuota(userRole string) RoleQuota {
	if q, ok := s.quotas[userRole]; ok {
		return q
	}
	// Safe default for unknown roles
	return RoleQuota{SessionLimit: 20, DailyLimit: 50}
}

// GetSession retrieves a session by ID. Negative IDs are routed to legacy chat_sessions.
func (s *AgentService) GetSession(ctx context.Context, id int64) (*model.AgentSession, error) {
	if id < 0 {
		return s.repo.GetLegacySession(ctx, id)
	}
	return s.repo.GetSession(ctx, id)
}

// ListSessions returns all agent sessions plus legacy chat sessions merged together.
func (s *AgentService) ListSessions(ctx context.Context, ownerID int64) ([]model.AgentSession, error) {
	sessions, err := s.repo.ListSessions(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("service: list sessions: %w", err)
	}
	legacy, err := s.repo.ListLegacySessions(ctx, ownerID)
	if err != nil {
		// Legacy table may not exist in all environments; log and continue
		return sessions, nil
	}
	return append(sessions, legacy...), nil
}

// CreateSession creates a new agent session.
func (s *AgentService) CreateSession(ctx context.Context, sess *model.AgentSession) error {
	if sess.Title == "" {
		sess.Title = "新对话"
	}
	if sess.ContextJSON == "" {
		sess.ContextJSON = "{}"
	}
	return s.repo.CreateSession(ctx, sess)
}

// DeleteSession deletes a session by ID.
func (s *AgentService) DeleteSession(ctx context.Context, id int64) error {
	return s.repo.DeleteSession(ctx, id)
}

// GetMessages returns messages for a session. Negative IDs route to legacy.
func (s *AgentService) GetMessages(ctx context.Context, sessionID int64, limit int) ([]model.AgentMessage, error) {
	if sessionID < 0 {
		return s.repo.GetLegacyMessages(ctx, sessionID, limit)
	}
	return s.repo.GetMessages(ctx, sessionID, limit)
}

// SaveUserMessage validates and saves a user message with quota enforcement.
// Uses student-level defaults for backward compatibility.
func (s *AgentService) SaveUserMessage(ctx context.Context, msg *model.AgentMessage) error {
	return s.SaveUserMessageWithRole(ctx, msg, "student")
}

// SaveUserMessageWithRole validates and saves a user message with role-based quota enforcement.
func (s *AgentService) SaveUserMessageWithRole(ctx context.Context, msg *model.AgentMessage, userRole string) error {
	if len(msg.Content) > maxAgentMessageLength {
		return fmt.Errorf("message exceeds max length of %d", maxAgentMessageLength)
	}

	quota := s.GetQuota(userRole)

	// Session message limit
	count, err := s.repo.CountSessionMessages(ctx, msg.SessionID)
	if err != nil {
		return fmt.Errorf("service: count session messages: %w", err)
	}
	if count >= quota.SessionLimit {
		return fmt.Errorf("session message limit reached (%d)", quota.SessionLimit)
	}

	// Daily message limit — need owner ID from session
	sess, err := s.GetSession(ctx, msg.SessionID)
	if err != nil {
		return fmt.Errorf("service: get session for daily check: %w", err)
	}
	dailyCount, err := s.repo.CountTodayMessages(ctx, sess.OwnerID)
	if err != nil {
		return fmt.Errorf("service: count daily messages: %w", err)
	}
	if dailyCount >= quota.DailyLimit {
		return fmt.Errorf("daily message limit reached (%d)", quota.DailyLimit)
	}

	msg.Role = "user"
	return s.repo.CreateMessage(ctx, msg)
}

// SaveAssistantMessage saves an assistant message (no quota checks).
func (s *AgentService) SaveAssistantMessage(ctx context.Context, msg *model.AgentMessage) error {
	msg.Role = "assistant"
	return s.repo.CreateMessage(ctx, msg)
}

// UpdateContext updates the session's context JSON.
func (s *AgentService) UpdateContext(ctx context.Context, sessionID int64, contextJSON string) error {
	return s.repo.UpdateSessionContext(ctx, sessionID, contextJSON)
}

// CheckSensitiveContent returns true if the message contains any sensitive keywords.
func CheckSensitiveContent(message string) bool {
	lower := strings.ToLower(message)
	for _, kw := range sensitiveAgentKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ValidateContextSwitch checks whether a context switch is allowed.
// If the current and new context differ, force must be true.
func ValidateContextSwitch(currentJSON, newJSON string, force bool) error {
	if currentJSON == "" || currentJSON == "{}" {
		return nil // no existing context, nothing to switch
	}
	if currentJSON == newJSON {
		return nil // same context
	}

	// Deep compare via unmarshal
	var current, updated map[string]any
	_ = json.Unmarshal([]byte(currentJSON), &current)
	_ = json.Unmarshal([]byte(newJSON), &updated)

	if contextEqual(current, updated) {
		return nil
	}

	if !force {
		return fmt.Errorf("context switch requires force_context_switch=true")
	}
	return nil
}

// contextEqual does a shallow comparison of two context maps.
func contextEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		// Compare as JSON strings for simplicity
		ja, _ := json.Marshal(va)
		jb, _ := json.Marshal(vb)
		if string(ja) != string(jb) {
			return false
		}
	}
	return true
}
