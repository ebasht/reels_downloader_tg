-- +goose Up
-- Bot messages scheduled for automatic deletion.
CREATE TABLE bot_messages (
    chat_id     BIGINT NOT NULL,
    message_id  INTEGER NOT NULL,
    delete_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (chat_id, message_id)
);

CREATE INDEX bot_messages_delete_at_idx ON bot_messages (delete_at);

-- +goose Down
DROP TABLE bot_messages;
