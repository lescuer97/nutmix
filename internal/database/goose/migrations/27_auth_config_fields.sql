-- +goose Up
ALTER TABLE config ADD mint_require_auth boolean;
ALTER TABLE config ADD mint_auth_discovery_url text;
ALTER TABLE config ADD mint_auth_oicd_client_id text;
ALTER TABLE config ADD mint_auth_rate_limit_per_minute int4;
ALTER TABLE config ADD mint_auth_max_blind_tokens int4;



-- +goose Down
ALTER TABLE config DROP COLUMN mint_require_auth;
ALTER TABLE config DROP COLUMN mint_auth_discovery_url;
ALTER TABLE config DROP COLUMN mint_auth_oicd_client_id;
ALTER TABLE config DROP COLUMN mint_auth_rate_limit_per_minute;
ALTER TABLE config DROP COLUMN mint_auth_max_blind_tokens;

