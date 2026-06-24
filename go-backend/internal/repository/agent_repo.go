// Package repository provides data access implementations.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

// agentRepo implements AgentRepo backed by SQLite.
type agentRepo struct {
	db *store.DB
}

// NewAgentRepo creates a new AgentRepo.
func NewAgentRepo(db *store.DB) AgentRepo {
	return &agentRepo{db: db}
}

func (r *agentRepo) GetSession(ctx context.Context, id int64) (*model.AgentSession, error) {
	s := &model.AgentSession{}
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at
		 FROM agent_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.OwnerID, &s.OwnerRole, &s.AgentRole, &s.Title, &s.ContextJSON, &s.CreatedAt, &s.LastActiveAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("repo: agent session %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("repo: get agent session: %w", err)
	}
	return s, nil
}

func (r *agentRepo) ListSessions(ctx context.Context, ownerID int64) ([]model.AgentSession, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at
		 FROM agent_sessions WHERE owner_id = ? ORDER BY last_active_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("repo: list agent sessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.AgentSession
	for rows.Next() {
		var s model.AgentSession
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.OwnerRole, &s.AgentRole, &s.Title, &s.ContextJSON, &s.CreatedAt, &s.LastActiveAt); err != nil {
			return nil, fmt.Errorf("repo: scan agent session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *agentRepo) CreateSession(ctx context.Context, s *model.AgentSession) error {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO agent_sessions (owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.OwnerID, s.OwnerRole, s.AgentRole, s.Title, s.ContextJSON, now, now)
	if err != nil {
		return fmt.Errorf("repo: create agent session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repo: get session id: %w", err)
	}
	s.ID = id
	s.CreatedAt = now
	s.LastActiveAt = now
	return nil
}

func (r *agentRepo) DeleteSession(ctx context.Context, id int64) error {
	if _, err := r.db.Writer.ExecContext(ctx, `DELETE FROM agent_messages WHERE session_id = ?`, id); err != nil {
		return fmt.Errorf("repo: delete agent messages: %w", err)
	}
	if _, err := r.db.Writer.ExecContext(ctx, `DELETE FROM agent_sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("repo: delete agent session: %w", err)
	}
	return nil
}

func (r *agentRepo) GetMessages(ctx context.Context, sessionID int64, limit int) ([]model.AgentMessage, error) {
	query := `SELECT id, session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at
			  FROM agent_messages WHERE session_id = ? ORDER BY created_at ASC`
	args := []any{sessionID}
	if limit > 0 {
		query = `SELECT id, session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at
				 FROM agent_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ?`
		args = append(args, limit)
	}
	rows, err := r.db.Reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: get agent messages: %w", err)
	}
	defer rows.Close()

	var messages []model.AgentMessage
	for rows.Next() {
		var m model.AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.ToolCallID, &m.ToolName, &m.PromptTokens, &m.CompletionTokens, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan agent message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (r *agentRepo) CreateMessage(ctx context.Context, m *model.AgentMessage) error {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO agent_messages (session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.SessionID, m.Role, m.Content, m.ToolCallID, m.ToolName, m.PromptTokens, m.CompletionTokens, now)
	if err != nil {
		return fmt.Errorf("repo: create agent message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repo: get message id: %w", err)
	}
	m.ID = id
	m.CreatedAt = now

	// Update session last_active_at
	_, _ = r.db.Writer.ExecContext(ctx,
		`UPDATE agent_sessions SET last_active_at = ? WHERE id = ?`, now, m.SessionID)
	return nil
}

func (r *agentRepo) CountTodayMessages(ctx context.Context, ownerID int64) (int, error) {
	today := time.Now().Format("2006-01-02")
	var count int
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agent_messages am
		 JOIN agent_sessions s ON am.session_id = s.id
		 WHERE s.owner_id = ? AND am.role = 'user' AND am.created_at >= ?`, ownerID, today).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("repo: count today messages: %w", err)
	}
	return count, nil
}

func (r *agentRepo) CountSessionMessages(ctx context.Context, sessionID int64) (int, error) {
	var count int
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agent_messages WHERE session_id = ?`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("repo: count session messages: %w", err)
	}
	return count, nil
}

func (r *agentRepo) UpdateSessionContext(ctx context.Context, sessionID int64, contextJSON string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Writer.ExecContext(ctx,
		`UPDATE agent_sessions SET context_json = ?, last_active_at = ? WHERE id = ?`,
		contextJSON, now, sessionID)
	if err != nil {
		return fmt.Errorf("repo: update session context: %w", err)
	}
	return nil
}

// --- Legacy chat_sessions backward compatibility ---

func (r *agentRepo) ListLegacySessions(ctx context.Context, ownerID int64) ([]model.AgentSession, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, student_id, title, created_at, last_active_at
		 FROM chat_sessions WHERE student_id = ? AND is_deleted = 0 AND migrated_at IS NULL ORDER BY last_active_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("repo: list legacy sessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.AgentSession
	for rows.Next() {
		var s model.AgentSession
		var realID int64
		if err := rows.Scan(&realID, &s.OwnerID, &s.Title, &s.CreatedAt, &s.LastActiveAt); err != nil {
			return nil, fmt.Errorf("repo: scan legacy session: %w", err)
		}
		s.ID = -realID
		s.OwnerRole = "student"
		s.AgentRole = "student"
		s.ContextJSON = "{}"
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *agentRepo) GetLegacySession(ctx context.Context, id int64) (*model.AgentSession, error) {
	realID := -id
	s := &model.AgentSession{}
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, student_id, title, created_at, last_active_at
		 FROM chat_sessions WHERE id = ? AND is_deleted = 0 AND migrated_at IS NULL`, realID,
	).Scan(&s.ID, &s.OwnerID, &s.Title, &s.CreatedAt, &s.LastActiveAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("repo: legacy session %d not found", realID)
	}
	if err != nil {
		return nil, fmt.Errorf("repo: get legacy session: %w", err)
	}
	s.ID = -s.ID
	s.OwnerRole = "student"
	s.AgentRole = "student"
	s.ContextJSON = "{}"
	return s, nil
}

func (r *agentRepo) GetLegacyMessages(ctx context.Context, sessionID int64, limit int) ([]model.AgentMessage, error) {
	realID := -sessionID
	query := `SELECT id, session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at
			  FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC`
	args := []any{realID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := r.db.Reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: get legacy messages: %w", err)
	}
	defer rows.Close()

	var messages []model.AgentMessage
	for rows.Next() {
		var m model.AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.ToolCallID, &m.ToolName, &m.PromptTokens, &m.CompletionTokens, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan legacy message: %w", err)
		}
		m.SessionID = -m.SessionID
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
