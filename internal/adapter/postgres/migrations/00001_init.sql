-- +goose Up
CREATE TABLE users (
    id          BIGINT PRIMARY KEY,
    username    TEXT,
    first_name  TEXT NOT NULL DEFAULT '',
    last_name   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chats (
    id          BIGINT PRIMARY KEY,
    type        TEXT NOT NULL,
    title       TEXT,
    username    TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    added_by    BIGINT REFERENCES users (id),
    added_at    TIMESTAMPTZ,
    removed_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chats_added_by_idx ON chats (added_by);

-- Full history of the bot being added to / removed from chats.
CREATE TABLE chat_membership_events (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id     BIGINT REFERENCES users (id),
    old_status  TEXT NOT NULL,
    new_status  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chat_membership_events_chat_id_idx ON chat_membership_events (chat_id);

-- +goose Down
DROP TABLE chat_membership_events;
DROP TABLE chats;
DROP TABLE users;
