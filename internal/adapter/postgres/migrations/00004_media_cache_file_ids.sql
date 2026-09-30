-- +goose Up
-- A carousel post is several photos, so the cache keeps a list of file IDs.
ALTER TABLE media_cache ADD COLUMN file_ids TEXT[] NOT NULL DEFAULT '{}';
UPDATE media_cache SET file_ids = ARRAY[file_id];
ALTER TABLE media_cache DROP COLUMN file_id;

-- +goose Down
ALTER TABLE media_cache ADD COLUMN file_id TEXT NOT NULL DEFAULT '';
UPDATE media_cache SET file_id = COALESCE(file_ids[1], '');
ALTER TABLE media_cache DROP COLUMN file_ids;
