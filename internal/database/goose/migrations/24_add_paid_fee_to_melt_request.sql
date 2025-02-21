-- +goose Up
ALTER TABLE melt_request ADD fee_paid int8;
UPDATE melt_request SET fee_paid = 0 WHERE fee_paid IS NULL;



-- +goose Down
ALTER TABLE melt_request DROP COLUMN fee_paid;
