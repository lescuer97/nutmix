-- +goose Up
ALTER TABLE melt_request ADD fee_paid int8;



-- +goose Down
ALTER TABLE melt_request DROP COLUMN fee_paid;
