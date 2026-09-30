-- +goose Up
ALTER TABLE chats ADD COLUMN listings_downloaded INTEGER NOT NULL DEFAULT 0;

ALTER TABLE downloads DROP CONSTRAINT downloads_media_type_check;
ALTER TABLE downloads ADD CONSTRAINT downloads_media_type_check
    CHECK (media_type IN ('reel', 'post', 'listing'));

-- +goose Down
DELETE FROM downloads WHERE media_type = 'listing';
ALTER TABLE downloads DROP CONSTRAINT downloads_media_type_check;
ALTER TABLE downloads ADD CONSTRAINT downloads_media_type_check
    CHECK (media_type IN ('reel', 'post'));

ALTER TABLE chats DROP COLUMN listings_downloaded;
