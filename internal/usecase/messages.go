package usecase

import (
	"context"
	"fmt"
	"time"

	"video_download_bot/internal/domain"
)

type MessageRepository interface {
	Schedule(ctx context.Context, chatID int64, messageIDs []int, deleteAt time.Time) error
	Due(ctx context.Context, limit int) ([]domain.BotMessage, error)
	Remove(ctx context.Context, chatID int64, messageIDs []int) error
}

// MessageService schedules bot messages for later deletion.
type MessageService struct {
	repo MessageRepository
	ttl  time.Duration
	now  func() time.Time
}

func NewMessageService(repo MessageRepository, ttl time.Duration) *MessageService {
	return &MessageService{repo: repo, ttl: ttl, now: time.Now}
}

// Schedule marks the given messages for deletion after the configured TTL.
func (s *MessageService) Schedule(ctx context.Context, chatID int64, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if err := s.repo.Schedule(ctx, chatID, messageIDs, s.now().Add(s.ttl)); err != nil {
		return fmt.Errorf("schedule messages in chat %d: %w", chatID, err)
	}
	return nil
}

// Due returns messages that are ready to be deleted.
func (s *MessageService) Due(ctx context.Context, limit int) ([]domain.BotMessage, error) {
	return s.repo.Due(ctx, limit)
}

// Remove drops messages from the schedule after a delete attempt.
func (s *MessageService) Remove(ctx context.Context, chatID int64, messageIDs []int) error {
	return s.repo.Remove(ctx, chatID, messageIDs)
}
