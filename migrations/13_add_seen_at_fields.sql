-- +goose Up
ALTER TABLE mint_request ADD seen_at int NOT NULL DEFAULT 0;
ALTER TABLE melt_request ADD seen_at int NOT NULL DEFAULT 0;
ALTER TABLE proofs ADD seen_at int NOT NULL DEFAULT 0;



-- +goose Down
ALTER TABLE mint_request DROP COLUMN seen_at;
ALTER TABLE melt_request DROP COLUMN seen_at;
ALTER TABLE proofs DROP COLUMN seen_at;
