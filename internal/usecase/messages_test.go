package usecase

import (
	"context"
	"testing"
	"time"

	"video_download_bot/internal/domain"
)

type memoryMessages struct {
	scheduled []scheduled
	due       []domain.BotMessage
	removed   []domain.BotMessage
}

type scheduled struct {
	chatID int64
	ids    []int
	at     time.Time
}

func (m *memoryMessages) Schedule(_ context.Context, chatID int64, messageIDs []int, deleteAt time.Time) error {
	ids := append([]int(nil), messageIDs...)
	m.scheduled = append(m.scheduled, scheduled{chatID: chatID, ids: ids, at: deleteAt})
	return nil
}

func (m *memoryMessages) Due(_ context.Context, limit int) ([]domain.BotMessage, error) {
	if limit > len(m.due) {
		limit = len(m.due)
	}
	out := append([]domain.BotMessage(nil), m.due[:limit]...)
	return out, nil
}

func (m *memoryMessages) Remove(_ context.Context, chatID int64, messageIDs []int) error {
	for _, id := range messageIDs {
		m.removed = append(m.removed, domain.BotMessage{ChatID: chatID, MessageID: id})
	}
	return nil
}

func TestMessageServiceScheduleUsesTTL(t *testing.T) {
	repo := &memoryMessages{}
	svc := NewMessageService(repo, 72*time.Hour)
	fixed := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	if err := svc.Schedule(context.Background(), -100, []int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if len(repo.scheduled) != 1 {
		t.Fatalf("scheduled = %d, want 1", len(repo.scheduled))
	}
	got := repo.scheduled[0]
	if got.chatID != -100 || len(got.ids) != 2 || !got.at.Equal(fixed.Add(72*time.Hour)) {
		t.Fatalf("got %+v", got)
	}
}

func TestMessageServiceScheduleEmpty(t *testing.T) {
	repo := &memoryMessages{}
	svc := NewMessageService(repo, time.Hour)
	if err := svc.Schedule(context.Background(), 1, nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.scheduled) != 0 {
		t.Fatalf("unexpected schedule: %+v", repo.scheduled)
	}
}
