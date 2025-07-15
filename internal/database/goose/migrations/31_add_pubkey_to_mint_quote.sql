-- +goose Up
ALTER TABLE mint_request ADD pubkey bytea;



-- +goose Down
ALTER TABLE mint_request DROP COLUMN pubkey;
