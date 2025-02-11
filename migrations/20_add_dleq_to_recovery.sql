-- +goose Up
ALTER TABLE recovery_signature ADD dleq_e text;
ALTER TABLE recovery_signature ADD dleq_s text;



-- +goose Down
ALTER TABLE recovery_signature DROP COLUMN dleq_e;
ALTER TABLE recovery_signature DROP COLUMN dleq_s;
