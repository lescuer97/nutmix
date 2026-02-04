-- +goose Up
ALTER TABLE mint_request DROP COLUMN request_paid;
ALTER TABLE melt_request DROP COLUMN request_paid;


-- +goose Down
ALTER TABLE mint_request ADD COLUMN request_paid BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE mint_request ALTER COLUMN request_paid DROP DEFAULT;
ALTER TABLE melt_request ADD COLUMN request_paid BOOLEAN;
