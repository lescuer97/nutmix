-- +goose Up
ALTER TABLE seeds ADD input_fee_ppk int NOT NULL DEFAULT 0;



-- +goose Down
ALTER TABLE seeds DROP COLUMN input_fee_ppk;
