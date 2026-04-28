-- +goose Up
CREATE TABLE nostr_notification_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    nostr_notification_npubs BYTEA[],
    nostr_notifications BOOLEAN NOT NULL DEFAULT FALSE,
    nostr_notification_nip04_dm BOOLEAN NOT NULL DEFAULT FALSE
);

-- +goose Down
DROP TABLE IF EXISTS nostr_notification_config;
