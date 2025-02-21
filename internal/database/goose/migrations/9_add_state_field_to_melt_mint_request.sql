-- +goose Up
ALTER TABLE mint_request ADD state TEXT;
ALTER TABLE melt_request ADD state TEXT;


-- +goose Down
ALTER TABLE mint_request DROP COLUMN state;
ALTER TABLE melt_request DROP COLUMN state;

