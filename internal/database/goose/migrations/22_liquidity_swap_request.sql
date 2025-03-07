-- +goose Up
CREATE TABLE liquidity_swaps(
    amount INTEGER,
    id TEXT,
    state TEXT,
    type TEXT,
	expiration int4 NOT NULL,
    lightning_invoice TEXT NOT NULL
);


-- +goose Down
DROP TABLE IF EXISTS liquidity_swaps;
