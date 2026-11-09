package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"video_download_bot/internal/domain"
)

type DeletionRepository struct {
	pool *pgxpool.Pool
}

func NewDeletionRepository(pool *pgxpool.Pool) *DeletionRepository {
	return &DeletionRepository{pool: pool}
}

func (r *DeletionRepository) ScheduleDeletion(ctx context.Context, chatID int64, messageIDs []int, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scheduled_deletions (chat_id, message_id, delete_at)
		SELECT $1, id, $3 FROM unnest($2::int[]) AS id
		ON CONFLICT (chat_id, message_id) DO UPDATE SET delete_at = EXCLUDED.delete_at`,
		chatID, messageIDs, at,
	)
	if err != nil {
		return fmt.Errorf("insert scheduled deletions: %w", err)
	}
	return nil
}

func (r *DeletionRepository) DueDeletions(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledDeletion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT chat_id, message_id, delete_at FROM scheduled_deletions
		WHERE delete_at <= $1 ORDER BY delete_at LIMIT $2`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query due deletions: %w", err)
	}
	due, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ScheduledDeletion, error) {
		var d domain.ScheduledDeletion
		err := row.Scan(&d.ChatID, &d.MessageID, &d.At)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan due deletions: %w", err)
	}
	return due, nil
}

func (r *DeletionRepository) RemoveDeletion(ctx context.Context, chatID int64, messageID int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM scheduled_deletions WHERE chat_id = $1 AND message_id = $2`, chatID, messageID)
	if err != nil {
		return fmt.Errorf("delete scheduled deletion: %w", err)
	}
	return nil
}
