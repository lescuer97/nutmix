-- +goose Up
ALTER TABLE mint_request ADD amount int4;



-- +goose Down
ALTER TABLE mint_request DROP COLUMN amount;
