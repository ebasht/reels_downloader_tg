-- +goose Up
-- Carousels mix photos and videos; video_items[i] marks file_ids[i] as a video.
-- Rows cached before this hold photos only and keep an empty array.
ALTER TABLE media_cache ADD COLUMN video_items BOOLEAN[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE media_cache DROP COLUMN video_items;
