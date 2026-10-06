-- +goose Up
-- Media is downloaded fresh for every link; nothing is kept after sending.
DROP TABLE media_cache;

-- +goose Down
CREATE TABLE media_cache (
    shortcode   TEXT PRIMARY KEY,
    media_type  TEXT NOT NULL CHECK (media_type IN ('reel', 'post')),
    file_ids    TEXT[] NOT NULL DEFAULT '{}',
    video_items BOOLEAN[] NOT NULL DEFAULT '{}',
    caption     TEXT NOT NULL DEFAULT '',
    width       INTEGER NOT NULL DEFAULT 0,
    height      INTEGER NOT NULL DEFAULT 0,
    duration    INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
