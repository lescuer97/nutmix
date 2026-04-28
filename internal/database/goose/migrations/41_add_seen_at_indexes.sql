-- +goose Up
CREATE INDEX IF NOT EXISTS idx_mint_request_seen_at ON mint_request (seen_at);
CREATE INDEX IF NOT EXISTS idx_melt_request_seen_at ON melt_request (seen_at);

-- +goose Down
DROP INDEX IF EXISTS idx_mint_request_seen_at;
DROP INDEX IF EXISTS idx_melt_request_seen_at;
