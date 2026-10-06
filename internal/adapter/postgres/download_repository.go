package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"video_download_bot/internal/domain"
)

type DownloadRepository struct {
	pool *pgxpool.Pool
}

func NewDownloadRepository(pool *pgxpool.Pool) *DownloadRepository {
	return &DownloadRepository{pool: pool}
}

func (r *DownloadRepository) SaveDownload(ctx context.Context, d domain.Download) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var userID *int64
		if d.User.ID != 0 {
			if err := upsertUser(ctx, tx, d.User); err != nil {
				return err
			}
			userID = &d.User.ID
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO downloads (chat_id, user_id, url, shortcode, media_type, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			d.ChatID, userID, d.URL, d.Shortcode, string(d.Type), d.At,
		)
		if err != nil {
			return fmt.Errorf("insert download: %w", err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE chats SET
				reels_downloaded = reels_downloaded + CASE WHEN $2::text = 'reel' THEN 1 ELSE 0 END,
				posts_downloaded = posts_downloaded + CASE WHEN $2::text = 'post' THEN 1 ELSE 0 END,
				listings_downloaded = listings_downloaded + CASE WHEN $2::text = 'listing' THEN 1 ELSE 0 END,
				updated_at       = now()
			WHERE id = $1`,
			d.ChatID, string(d.Type),
		)
		if err != nil {
			return fmt.Errorf("increment chat counters: %w", err)
		}
		return nil
	})
}
