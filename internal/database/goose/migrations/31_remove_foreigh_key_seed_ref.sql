-- +goose Up
ALTER TABLE proofs DROP CONSTRAINT proofs_seeds_fk;

-- +goose Down
ALTER TABLE proofs ADD CONSTRAINT proofs_seeds_fk FOREIGN KEY (id) REFERENCES seeds(id)