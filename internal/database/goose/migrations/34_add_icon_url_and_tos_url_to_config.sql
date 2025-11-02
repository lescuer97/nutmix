-- +goose Up
-- SQL in this section is executed when the migration is applied.
ALTER TABLE config ADD COLUMN icon_url TEXT;
ALTER TABLE config ADD COLUMN tos_url TEXT;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
ALTER TABLE config DROP COLUMN IF EXISTS icon_url;
ALTER TABLE config DROP COLUMN IF EXISTS tos_url;
