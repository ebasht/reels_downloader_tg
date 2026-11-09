package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"video_download_bot/internal/domain"
)

type DeletionRepository interface {
	ScheduleDeletion(ctx context.Context, chatID int64, messageIDs []int, at time.Time) error
	// DueDeletions returns up to limit deletions scheduled at or before now, oldest first.
	DueDeletions(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledDeletion, error)
	RemoveDeletion(ctx context.Context, chatID int64, messageID int) error
}

type MessageDeleter interface {
	// DeleteMessage returns domain.ErrMessageGone when retrying is pointless.
	DeleteMessage(chatID int64, messageID int) error
}

const (
	cleanupBatch = 100
	// cleanupGiveUp drops deletions that keep failing, e.g. during a long
	// Telegram outage, before the message outlives Telegram's 48-hour limit.
	cleanupGiveUp = 6 * time.Hour
)

// CleanupService deletes the bot's messages a fixed time after sending.
type CleanupService struct {
	repo DeletionRepository
	ttl  time.Duration
	now  func() time.Time
}

// NewCleanupService keeps messages for ttl; zero ttl disables cleanup.
func NewCleanupService(repo DeletionRepository, ttl time.Duration) *CleanupService {
	return &CleanupService{repo: repo, ttl: ttl, now: time.Now}
}

func (s *CleanupService) Enabled() bool { return s.ttl > 0 }

// Schedule queues the sent messages for deletion after the TTL.
func (s *CleanupService) Schedule(ctx context.Context, chatID int64, messageIDs []int) error {
	if !s.Enabled() || len(messageIDs) == 0 {
		return nil
	}
	if err := s.repo.ScheduleDeletion(ctx, chatID, messageIDs, s.now().Add(s.ttl)); err != nil {
		return fmt.Errorf("schedule deletion in chat %d: %w", chatID, err)
	}
	return nil
}

// Run deletes due messages every interval until ctx is cancelled.
func (s *CleanupService) Run(ctx context.Context, deleter MessageDeleter, interval time.Duration) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.deleteDue(ctx, deleter)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *CleanupService) deleteDue(ctx context.Context, deleter MessageDeleter) {
	now := s.now()
	due, err := s.repo.DueDeletions(ctx, now, cleanupBatch)
	if err != nil {
		log.Printf("due deletions: %v", err)
		return
	}
	for _, d := range due {
		err := deleter.DeleteMessage(d.ChatID, d.MessageID)
		if err != nil && !errors.Is(err, domain.ErrMessageGone) && now.Sub(d.At) < cleanupGiveUp {
			log.Printf("delete message %d in chat %d, will retry: %v", d.MessageID, d.ChatID, err)
			continue
		}
		if err != nil {
			log.Printf("delete message %d in chat %d, giving up: %v", d.MessageID, d.ChatID, err)
		}
		if err := s.repo.RemoveDeletion(ctx, d.ChatID, d.MessageID); err != nil {
			log.Printf("remove deletion: %v", err)
		}
	}
}
