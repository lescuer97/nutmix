-- +goose Up
CREATE TABLE liquidity_swaps(
    Amount INTEGER,
    Id TEXT,
    Destination TEXT,
    State TEXT,
    Type TEXT,
	Expiration int4 NOT NULL
);


-- +goose Down
DROP TABLE IF EXISTS liquidity_swaps;
