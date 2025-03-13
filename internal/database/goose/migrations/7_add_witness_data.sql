-- +goose Up
ALTER TABLE proofs ADD witness TEXT;
ALTER TABLE recovery_signature ADD witness TEXT;



-- +goose Down
ALTER TABLE proofs DROP COLUMN witness;
ALTER TABLE recovery_signature DROP COLUMN witness;
