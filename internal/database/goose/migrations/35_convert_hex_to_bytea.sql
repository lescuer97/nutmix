-- +goose Up
ALTER TABLE proofs ALTER COLUMN c TYPE bytea USING decode(c, 'hex');
ALTER TABLE proofs ALTER COLUMN y TYPE bytea USING decode(y, 'hex');
ALTER TABLE recovery_signature ALTER COLUMN "B_" TYPE bytea USING decode("B_", 'hex');
ALTER TABLE recovery_signature ALTER COLUMN "C_" TYPE bytea USING decode("C_", 'hex');
ALTER TABLE recovery_signature ALTER COLUMN dleq_e TYPE bytea USING decode(dleq_e, 'hex');
ALTER TABLE recovery_signature ALTER COLUMN dleq_s TYPE bytea USING decode(dleq_s, 'hex');
ALTER TABLE melt_change_message ALTER COLUMN "B_" TYPE bytea USING decode("B_", 'hex');

-- +goose Down
ALTER TABLE proofs ALTER COLUMN c TYPE text USING encode(c, 'hex');
ALTER TABLE proofs ALTER COLUMN y TYPE text USING encode(y, 'hex');
ALTER TABLE recovery_signature ALTER COLUMN "B_" TYPE text USING encode("B_", 'hex');
ALTER TABLE recovery_signature ALTER COLUMN "C_" TYPE text USING encode("C_", 'hex');
ALTER TABLE recovery_signature ALTER COLUMN dleq_e TYPE text USING encode(dleq_e, 'hex');
ALTER TABLE recovery_signature ALTER COLUMN dleq_s TYPE text USING encode(dleq_s, 'hex');
ALTER TABLE melt_change_message ALTER COLUMN "B_" TYPE text USING encode("B_", 'hex');

