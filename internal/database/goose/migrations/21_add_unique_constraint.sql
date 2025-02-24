-- +goose Up
ALTER TABLE proofs ADD CONSTRAINT unique_y UNIQUE (secret, y);
ALTER TABLE recovery_signature ADD CONSTRAINT unique_recovery_B_ UNIQUE ("B_");



-- +goose Down
ALTER TABLE proofs DROP CONSTRAINT unique_y;
ALTER TABLE recovery_signature DROP CONSTRAINT unique_recovery_B_;

