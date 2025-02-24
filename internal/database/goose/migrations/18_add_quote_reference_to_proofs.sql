-- +goose Up
ALTER TABLE proofs
ADD COLUMN quote text;


-- +goose Down
ALTER TABLE proofs
DROP COLUMN quote;

