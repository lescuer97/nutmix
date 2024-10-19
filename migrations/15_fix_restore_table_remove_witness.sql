-- +goose Up
ALTER TABLE recovery_signature DROP COLUMN witness;



-- +goose Down
ALTER TABLE recovery_signature ADD witness TEXT;