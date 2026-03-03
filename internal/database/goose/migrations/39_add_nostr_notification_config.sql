-- +goose Up
ALTER TABLE config ADD COLUMN nostr_notification_npubs BYTEA[];
ALTER TABLE config ADD COLUMN nostr_notifications BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE config ADD COLUMN nostr_notification_nip04_dm BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE config DROP COLUMN IF EXISTS nostr_notification_npubs;
ALTER TABLE config DROP COLUMN IF EXISTS nostr_notifications;
ALTER TABLE config DROP COLUMN IF EXISTS nostr_notification_nip04_dm;
