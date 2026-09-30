-- +goose Up
ALTER TABLE chats
    ADD COLUMN reels_downloaded INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN posts_downloaded INTEGER NOT NULL DEFAULT 0;

CREATE TABLE downloads (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id     BIGINT REFERENCES users (id),
    url         TEXT NOT NULL,
    shortcode   TEXT NOT NULL,
    media_type  TEXT NOT NULL CHECK (media_type IN ('reel', 'post')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX downloads_chat_id_idx ON downloads (chat_id);
CREATE INDEX downloads_shortcode_idx ON downloads (shortcode);

-- +goose Down
DROP TABLE downloads;

ALTER TABLE chats
    DROP COLUMN posts_downloaded,
    DROP COLUMN reels_downloaded;
