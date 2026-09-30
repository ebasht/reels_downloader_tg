package postgres

import (
	"context"
	"errors"
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

func (r *DownloadRepository) FindCached(ctx context.Context, shortcode string) (domain.Media, bool, error) {
	var (
		mediaType, caption      string
		fileIDs                 []string
		width, height, duration int
	)
	err := r.pool.QueryRow(ctx, `
		SELECT media_type, file_ids, caption, width, height, duration
		FROM media_cache WHERE shortcode = $1`,
		shortcode,
	).Scan(&mediaType, &fileIDs, &caption, &width, &height, &duration)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Media{}, false, nil
	}
	if err != nil {
		return domain.Media{}, false, fmt.Errorf("find cached media: %w", err)
	}

	if len(fileIDs) == 0 {
		return domain.Media{}, false, nil
	}
	if domain.MediaType(mediaType) == domain.MediaReel {
		return domain.Media{Cached: true, Video: &domain.Video{
			FileID: fileIDs[0], Width: width, Height: height, Duration: duration,
		}}, true, nil
	}
	return domain.Media{Cached: true, Post: &domain.Post{
		ImageFileIDs: fileIDs, Caption: caption,
	}}, true, nil
}

func (r *DownloadRepository) SaveCached(ctx context.Context, shortcode string, m domain.Media) error {
	var caption string
	var width, height, duration int
	if m.Video != nil {
		width, height, duration = m.Video.Width, m.Video.Height, m.Video.Duration
	}
	if m.Post != nil {
		caption = m.Post.Caption
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO media_cache (shortcode, media_type, file_ids, caption, width, height, duration)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (shortcode) DO UPDATE SET
			media_type = EXCLUDED.media_type,
			file_ids   = EXCLUDED.file_ids,
			caption    = EXCLUDED.caption,
			width      = EXCLUDED.width,
			height     = EXCLUDED.height,
			duration   = EXCLUDED.duration,
			updated_at = now()`,
		shortcode, string(m.Type()), m.FileIDs(), caption, width, height, duration,
	)
	if err != nil {
		return fmt.Errorf("save cached media: %w", err)
	}
	return nil
}

func (r *DownloadRepository) DeleteCached(ctx context.Context, shortcode string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM media_cache WHERE shortcode = $1`, shortcode); err != nil {
		return fmt.Errorf("delete cached media: %w", err)
	}
	return nil
}
