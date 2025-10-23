-- +goose Up
ALTER TABLE mint_request ADD description text;



-- +goose Down
ALTER TABLE mint_request DROP COLUMN description;
