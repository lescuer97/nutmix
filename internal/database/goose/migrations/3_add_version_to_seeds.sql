-- +goose Up
ALTER TABLE seeds ADD version INT;


-- +goose Down
ALTER TABLE seeds DROP COLUMN version
