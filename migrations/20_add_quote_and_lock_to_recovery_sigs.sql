-- +goose Up
ALTER TABLE recovery_signature ADD COLUMN quote text;
ALTER TABLE recovery_signature ADD COLUMN locked bool NOT NULL;


-- set all already created recovery_signatures to locked = false
UPDATE recovery_signature SET locked = false;



-- +goose Down
ALTER TABLE recovery_signature DROP COLUMN quote;
ALTER TABLE recovery_signature DROP COLUMN locked;
