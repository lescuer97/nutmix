-- +goose Up
ALTER TABLE melt_request ADD payment_preimage TEXT;


-- +goose Down
ALTER TABLE melt_request DROP COLUMN payment_preimage;

