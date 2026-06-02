package service

import (
	"context"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/sse"
)

// NotificationService handles notification operations.
type NotificationService struct {
	repo   repository.NotificationRepo
	broker *sse.Broker
}

// NewNotificationService creates a new notification service.
func NewNotificationService(repo repository.NotificationRepo, broker *sse.Broker) *NotificationService {
	return &NotificationService{repo: repo, broker: broker}
}

func (s *NotificationService) List(ctx context.Context, userID int64, unreadOnly bool, params repository.ListParams) ([]model.Notification, int64, error) {
	return s.repo.List(ctx, userID, unreadOnly, params)
}

func (s *NotificationService) MarkRead(ctx context.Context, id int64) error {
	return s.repo.MarkRead(ctx, id)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID int64) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// Send creates a notification and pushes it via SSE.
func (s *NotificationService) Send(ctx context.Context, n *model.Notification) error {
	if err := s.repo.Create(ctx, n); err != nil {
		return err
	}
	// Push via SSE
	if s.broker != nil {
		s.broker.Publish(sse.Event{
			UserID: n.UserID,
			Type:   "notification",
			Data:   `{"id":` + itoa(n.ID) + `,"type":"` + n.Type + `","title":"` + n.Title + `"}`,
		})
	}
	return nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
