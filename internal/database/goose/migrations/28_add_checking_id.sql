-- +goose Up
ALTER TABLE mint_request ADD checking_id text;
ALTER TABLE melt_request ADD checking_id text;
ALTER TABLE liquidity_swaps ADD checking_id text;
UPDATE mint_request SET checking_id = '' WHERE checking_id IS NULL;
UPDATE melt_request SET checking_id = '' WHERE checking_id IS NULL;
UPDATE liquidity_swaps SET checking_id = '' WHERE checking_id IS NULL;



-- +goose Down
ALTER TABLE mint_request DROP COLUMN checking_id;
ALTER TABLE melt_request DROP COLUMN checking_id;
ALTER TABLE liquidity_swaps DROP COLUMN checking_id;
