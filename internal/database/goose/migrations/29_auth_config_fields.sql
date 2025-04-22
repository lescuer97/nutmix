-- +goose Up
ALTER TABLE config ADD mint_require_auth boolean DEFAULT FALSE;
ALTER TABLE config ADD mint_auth_oicd_url text DEFAULT '';
ALTER TABLE config ADD mint_auth_oicd_client_id text DEFAULT '';
ALTER TABLE config ADD mint_auth_rate_limit_per_minute int4 default 5;
ALTER TABLE config ADD mint_auth_max_blind_tokens int4 default 100;
ALTER TABLE config ADD mint_auth_clear_auth_urls text[] default '{}';
ALTER TABLE config ADD mint_auth_blind_auth_urls text[] default '{}';



-- +goose Down
ALTER TABLE config DROP COLUMN mint_require_auth;
ALTER TABLE config DROP COLUMN mint_auth_oicd_url;
ALTER TABLE config DROP COLUMN mint_auth_oicd_client_id;
ALTER TABLE config DROP COLUMN mint_auth_rate_limit_per_minute;
ALTER TABLE config DROP COLUMN mint_auth_max_blind_tokens;
ALTER TABLE config DROP COLUMN mint_auth_clear_auth_urls;
ALTER TABLE config DROP COLUMN mint_auth_blind_auth_urls;

