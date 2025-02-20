
-- +goose Up
ALTER TABLE proofs ADD Y TEXT;


-- +goose Down
ALTER TABLE proofs DROP COLUMN Y
