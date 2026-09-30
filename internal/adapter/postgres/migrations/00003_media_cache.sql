-- +goose Up
-- Telegram file IDs of media already sent by the bot, so repeated links are
-- re-sent without downloading from Instagram again.
CREATE TABLE media_cache (
    shortcode   TEXT PRIMARY KEY,
    media_type  TEXT NOT NULL CHECK (media_type IN ('reel', 'post')),
    file_id     TEXT NOT NULL,
    caption     TEXT NOT NULL DEFAULT '',
    width       INTEGER NOT NULL DEFAULT 0,
    height      INTEGER NOT NULL DEFAULT 0,
    duration    INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE media_cache;
