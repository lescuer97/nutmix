
-- +goose Up
ALTER TABLE mint_request ADD unit TEXT;


-- +goose Down
ALTER TABLE mint_request DROP COLUMN unit;
