-- +goose Up
ALTER TABLE seeds ADD encrypted BOOL NOT NULL DEFAULT FALSE;



-- +goose Down
ALTER TABLE seeds DROP COLUMN encrypted;
