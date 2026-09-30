package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"video_download_bot/internal/domain"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

func (r *ChatRepository) RecordMembership(ctx context.Context, change domain.MembershipChange) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var actorID *int64
		if change.Actor.ID != 0 {
			if err := upsertUser(ctx, tx, change.Actor); err != nil {
				return err
			}
			actorID = &change.Actor.ID
		}

		c := change.Chat
		if change.NewStatus.IsPresent() {
			_, err := tx.Exec(ctx, `
				INSERT INTO chats (id, type, title, username, is_active, added_by, added_at, removed_at)
				VALUES ($1, $2, $3, $4, TRUE, $5, $6, NULL)
				ON CONFLICT (id) DO UPDATE SET
					type       = EXCLUDED.type,
					title      = EXCLUDED.title,
					username   = EXCLUDED.username,
					is_active  = TRUE,
					-- status changes inside the chat (e.g. promoted to admin) keep the original adder
					added_by   = CASE WHEN chats.is_active AND chats.added_by IS NOT NULL THEN chats.added_by ELSE EXCLUDED.added_by END,
					added_at   = CASE WHEN chats.is_active AND chats.added_by IS NOT NULL THEN chats.added_at ELSE EXCLUDED.added_at END,
					removed_at = NULL,
					updated_at = now()`,
				c.ID, c.Type, nullIfEmpty(c.Title), nullIfEmpty(c.Username), actorID, change.At,
			)
			if err != nil {
				return fmt.Errorf("upsert active chat: %w", err)
			}
		} else {
			_, err := tx.Exec(ctx, `
				INSERT INTO chats (id, type, title, username, is_active, removed_at)
				VALUES ($1, $2, $3, $4, FALSE, $5)
				ON CONFLICT (id) DO UPDATE SET
					type       = EXCLUDED.type,
					title      = EXCLUDED.title,
					username   = EXCLUDED.username,
					is_active  = FALSE,
					removed_at = EXCLUDED.removed_at,
					updated_at = now()`,
				c.ID, c.Type, nullIfEmpty(c.Title), nullIfEmpty(c.Username), change.At,
			)
			if err != nil {
				return fmt.Errorf("upsert inactive chat: %w", err)
			}
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO chat_membership_events (chat_id, user_id, old_status, new_status, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			c.ID, actorID, string(change.OldStatus), string(change.NewStatus), change.At,
		)
		if err != nil {
			return fmt.Errorf("insert membership event: %w", err)
		}
		return nil
	})
}

func (r *ChatRepository) EnsureChat(ctx context.Context, c domain.Chat) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO chats (id, type, title, username, is_active)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT (id) DO UPDATE SET
			type       = EXCLUDED.type,
			title      = EXCLUDED.title,
			username   = EXCLUDED.username,
			is_active  = TRUE,
			removed_at = NULL,
			updated_at = now()`,
		c.ID, c.Type, nullIfEmpty(c.Title), nullIfEmpty(c.Username),
	)
	return err
}

func upsertUser(ctx context.Context, tx pgx.Tx, u domain.User) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			username   = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name  = EXCLUDED.last_name,
			updated_at = now()`,
		u.ID, nullIfEmpty(u.Username), u.FirstName, nullIfEmpty(u.LastName),
	)
	if err != nil {
		return fmt.Errorf("upsert user %d: %w", u.ID, err)
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
