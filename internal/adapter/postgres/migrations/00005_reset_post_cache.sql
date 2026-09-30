-- +goose Up
-- Posts cached before carousel support hold only the first photo.
DELETE FROM media_cache WHERE media_type = 'post';

-- +goose Down
SELECT 1;
