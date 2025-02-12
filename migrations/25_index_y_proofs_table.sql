-- +goose Up
CREATE INDEX IF NOT EXISTS idx_proofs_y ON proofs (y);


-- +goose Down
DROP INDEX idx_proofs_y;
