package telegram

import (
	"context"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
)

const (
	cleanupInterval = time.Minute
	cleanupBatch    = 100
)

// runCleanup periodically deletes bot messages whose TTL has expired.
func (h *Handler) runCleanup(ctx context.Context) {
	if h.messages == nil {
		return
	}
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()

	h.purgeExpired(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.purgeExpired(ctx)
		}
	}
}

func (h *Handler) purgeExpired(ctx context.Context) {
	for {
		due, err := h.messages.Due(ctx, cleanupBatch)
		if err != nil {
			log.Printf("cleanup list: %v", err)
			return
		}
		if len(due) == 0 {
			return
		}

		byChat := groupByChat(due)
		for chatID, ids := range byChat {
			h.deleteScheduled(ctx, chatID, ids)
		}
		if len(due) < cleanupBatch {
			return
		}
	}
}

func (h *Handler) deleteScheduled(ctx context.Context, chatID int64, ids []int) {
	var keep []int
	for _, id := range ids {
		if err := h.deleteBotMessage(chatID, id); err != nil {
			if isTransientDeleteError(err) {
				log.Printf("cleanup delete chat %d msg %d: %v", chatID, id, err)
				keep = append(keep, id)
				continue
			}
			// Already gone, too old, or bot left the chat — drop the row.
			log.Printf("cleanup drop chat %d msg %d: %v", chatID, id, err)
		}
	}
	done := diffIDs(ids, keep)
	if err := h.messages.Remove(ctx, chatID, done); err != nil {
		log.Printf("cleanup remove: %v", err)
	}
}

func (h *Handler) deleteBotMessage(chatID int64, messageID int) error {
	_, err := h.api.Request(tgbotapi.NewDeleteMessage(chatID, messageID))
	return err
}

func (h *Handler) scheduleDelete(ctx context.Context, chatID int64, messageIDs []int) {
	if h.messages == nil || len(messageIDs) == 0 {
		return
	}
	if err := h.messages.Schedule(ctx, chatID, messageIDs); err != nil {
		log.Printf("schedule delete: %v", err)
	}
}

func groupByChat(msgs []domain.BotMessage) map[int64][]int {
	out := make(map[int64][]int, len(msgs))
	for _, m := range msgs {
		out[m.ChatID] = append(out[m.ChatID], m.MessageID)
	}
	return out
}

func diffIDs(all, keep []int) []int {
	if len(keep) == 0 {
		return all
	}
	skip := make(map[int]struct{}, len(keep))
	for _, id := range keep {
		skip[id] = struct{}{}
	}
	out := make([]int, 0, len(all)-len(keep))
	for _, id := range all {
		if _, ok := skip[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

// isTransientDeleteError reports errors worth retrying later (rate limits, network).
func isTransientDeleteError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "too many requests"),
		strings.Contains(s, "retry after"),
		strings.Contains(s, "timeout"),
		strings.Contains(s, "temporar"),
		strings.Contains(s, "connection"):
		return true
	default:
		return false
	}
}
