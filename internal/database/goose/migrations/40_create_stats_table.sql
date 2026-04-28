-- +goose Up
CREATE TABLE stats (
    id BIGSERIAL PRIMARY KEY,
    start_date BIGINT NOT NULL,
    end_date BIGINT NOT NULL,
    mint_summary JSONB NOT NULL,
    melt_summary JSONB NOT NULL,
    blind_sigs_summary JSONB NOT NULL,
    proofs_summary JSONB NOT NULL,
    fees BIGINT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS stats;
