-- +goose Up
-- Bot messages to delete from chats once delete_at has passed.
CREATE TABLE scheduled_deletions (
    chat_id    BIGINT      NOT NULL,
    message_id INTEGER     NOT NULL,
    delete_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (chat_id, message_id)
);
CREATE INDEX scheduled_deletions_delete_at_idx ON scheduled_deletions (delete_at);

-- +goose Down
DROP TABLE scheduled_deletions;
