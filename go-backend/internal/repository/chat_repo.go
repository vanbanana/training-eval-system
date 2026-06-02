package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteChatRepo struct {
	db *store.DB
}

func NewChatRepo(db *store.DB) ChatRepo {
	return &SQLiteChatRepo{db: db}
}

func (r *SQLiteChatRepo) GetSession(ctx context.Context, id int64) (*model.ChatSession, error) {
	var s model.ChatSession
	var isDeleted int
	var createdAt, lastActive sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id, student_id, evaluation_id, title, is_deleted, created_at, last_active_at
		 FROM chat_sessions WHERE id=?`, id).Scan(
		&s.ID, &s.StudentID, &s.EvaluationID, &s.Title, &isDeleted, &createdAt, &lastActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chat_repo: session not found")
		}
		return nil, err
	}
	s.IsDeleted = isDeleted != 0
	s.CreatedAt = parseTime(createdAt.String)
	s.LastActiveAt = parseTime(lastActive.String)
	return &s, nil
}

func (r *SQLiteChatRepo) ListSessions(ctx context.Context, studentID int64) ([]model.ChatSession, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, student_id, evaluation_id, title, is_deleted, created_at, last_active_at
		 FROM chat_sessions WHERE student_id=? AND is_deleted=0 ORDER BY last_active_at DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []model.ChatSession
	for rows.Next() {
		var s model.ChatSession
		var isDeleted int
		var createdAt, lastActive sql.NullString
		if err := rows.Scan(&s.ID, &s.StudentID, &s.EvaluationID, &s.Title, &isDeleted, &createdAt, &lastActive); err != nil {
			return nil, err
		}
		s.IsDeleted = isDeleted != 0
		s.CreatedAt = parseTime(createdAt.String)
		s.LastActiveAt = parseTime(lastActive.String)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *SQLiteChatRepo) CreateSession(ctx context.Context, s *model.ChatSession) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO chat_sessions (student_id, evaluation_id, title) VALUES (?, ?, ?)`,
		s.StudentID, s.EvaluationID, s.Title)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.ID = id
	return nil
}

func (r *SQLiteChatRepo) DeleteSession(ctx context.Context, id int64) error {
	_, err := r.db.Writer.ExecContext(ctx, "UPDATE chat_sessions SET is_deleted=1 WHERE id=?", id)
	return err
}

func (r *SQLiteChatRepo) GetMessages(ctx context.Context, sessionID int64, limit int) ([]model.ChatMessage, error) {
	rows, err := r.db.Reader.QueryContext(ctx,
		`SELECT id, session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at
		 FROM chat_messages WHERE session_id=? ORDER BY created_at ASC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		var createdAt sql.NullString
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.ToolCallID, &m.ToolName,
			&m.PromptTokens, &m.CompletionTokens, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(createdAt.String)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *SQLiteChatRepo) CreateMessage(ctx context.Context, m *model.ChatMessage) error {
	res, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO chat_messages (session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.SessionID, m.Role, m.Content, m.ToolCallID, m.ToolName, m.PromptTokens, m.CompletionTokens)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	// Update session last_active_at
	_, _ = r.db.Writer.ExecContext(ctx, "UPDATE chat_sessions SET last_active_at=datetime('now') WHERE id=?", m.SessionID)
	return nil
}

func (r *SQLiteChatRepo) CountTodayMessages(ctx context.Context, studentID int64) (int, error) {
	var count int
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages cm
		 JOIN chat_sessions cs ON cm.session_id=cs.id
		 WHERE cs.student_id=? AND cm.role='user' AND cm.created_at >= date('now')`, studentID).Scan(&count)
	return count, err
}

func (r *SQLiteChatRepo) CountSessionMessages(ctx context.Context, sessionID int64) (int, error) {
	var count int
	err := r.db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM chat_messages WHERE session_id=? AND role='user'", sessionID).Scan(&count)
	return count, err
}
