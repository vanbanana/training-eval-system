package service

import (
	"context"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

const (
	maxMessageLength   = 500
	maxSessionMessages = 20
	maxDailyMessages   = 50
)

// ChatService handles AI chat operations.
type ChatService struct {
	repo repository.ChatRepo
}

// NewChatService creates a new chat service.
func NewChatService(repo repository.ChatRepo) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) GetSession(ctx context.Context, id int64) (*model.ChatSession, error) {
	return s.repo.GetSession(ctx, id)
}

func (s *ChatService) ListSessions(ctx context.Context, studentID int64) ([]model.ChatSession, error) {
	return s.repo.ListSessions(ctx, studentID)
}

func (s *ChatService) CreateSession(ctx context.Context, session *model.ChatSession) error {
	return s.repo.CreateSession(ctx, session)
}

func (s *ChatService) DeleteSession(ctx context.Context, id int64) error {
	return s.repo.DeleteSession(ctx, id)
}

func (s *ChatService) GetMessages(ctx context.Context, sessionID int64) ([]model.ChatMessage, error) {
	return s.repo.GetMessages(ctx, sessionID, 100)
}

// SendMessage validates limits and creates a message.
func (s *ChatService) SendMessage(ctx context.Context, studentID int64, msg *model.ChatMessage) error {
	// Validate message length
	if len([]rune(msg.Content)) > maxMessageLength {
		return fmt.Errorf("chat_service: message exceeds %d characters", maxMessageLength)
	}

	// Check session message limit (20 rounds)
	sessionCount, err := s.repo.CountSessionMessages(ctx, msg.SessionID)
	if err != nil {
		return err
	}
	if sessionCount >= maxSessionMessages {
		return fmt.Errorf("chat_service: session message limit reached (%d)", maxSessionMessages)
	}

	// Check daily limit (50 messages/day)
	dailyCount, err := s.repo.CountTodayMessages(ctx, studentID)
	if err != nil {
		return err
	}
	if dailyCount >= maxDailyMessages {
		return fmt.Errorf("chat_service: daily message limit reached (%d)", maxDailyMessages)
	}

	return s.repo.CreateMessage(ctx, msg)
}
