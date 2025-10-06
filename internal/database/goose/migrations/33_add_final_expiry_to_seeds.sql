-- +goose Up
ALTER TABLE seeds ADD COLUMN final_expiry int4;

-- +goose Down
ALTER TABLE seeds DROP COLUMN final_expiry;
