-- +goose Up
ALTER TABLE mint_request RENAME COLUMN paid TO request_paid;
ALTER TABLE melt_request RENAME COLUMN paid TO request_paid;
ALTER TABLE mint_request ADD minted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE melt_request ADD melted BOOLEAN NOT NULL DEFAULT FALSE;


-- +goose Down
ALTER TABLE mint_request RENAME COLUMN request_paid TO paid;
ALTER TABLE melt_request RENAME COLUMN request_paid TO paid;
ALTER TABLE mint_request DROP COLUMN minted;
ALTER TABLE melt_request DROP COLUMN melted;

