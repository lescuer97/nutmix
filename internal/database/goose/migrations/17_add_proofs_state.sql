-- +goose Up
ALTER TABLE proofs ADD state TEXT;

-- if the proofs have null value on state asume they are spent
UPDATE proofs
SET state = 'SPENT'
WHERE state IS NULL;


-- +goose Down
ALTER TABLE proofs DROP COLUMN state
