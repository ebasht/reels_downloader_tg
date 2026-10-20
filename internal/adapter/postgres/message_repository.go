package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"video_download_bot/internal/domain"
)

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

// Schedule marks messages for deletion at deleteAt. Duplicates are ignored.
func (r *MessageRepository) Schedule(ctx context.Context, chatID int64, messageIDs []int, deleteAt time.Time) error {
	if len(messageIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bot_messages (chat_id, message_id, delete_at)
		SELECT $1, unnest($2::int[]), $3
		ON CONFLICT (chat_id, message_id) DO NOTHING`,
		chatID, messageIDs, deleteAt,
	)
	if err != nil {
		return fmt.Errorf("schedule %d messages in chat %d: %w", len(messageIDs), chatID, err)
	}
	return nil
}

// Due returns up to limit messages whose delete_at has passed, oldest first.
func (r *MessageRepository) Due(ctx context.Context, limit int) ([]domain.BotMessage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT chat_id, message_id
		FROM bot_messages
		WHERE delete_at <= now()
		ORDER BY delete_at
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list due bot messages: %w", err)
	}
	defer rows.Close()

	var out []domain.BotMessage
	for rows.Next() {
		var m domain.BotMessage
		if err := rows.Scan(&m.ChatID, &m.MessageID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Remove drops scheduled rows after a delete attempt (success, already gone, or undeletable).
func (r *MessageRepository) Remove(ctx context.Context, chatID int64, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		DELETE FROM bot_messages
		WHERE chat_id = $1 AND message_id = ANY ($2)`,
		chatID, messageIDs,
	)
	if err != nil {
		return fmt.Errorf("remove bot messages in chat %d: %w", chatID, err)
	}
	return nil
}
