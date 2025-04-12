-- +goose Up
ALTER TABLE config ADD strike_key text;
ALTER TABLE config ADD strike_endpoint text;
UPDATE config SET strike_key = '' WHERE strike_key IS NULL;
UPDATE config SET strike_endpoint = '' WHERE strike_endpoint IS NULL;



-- +goose Down
ALTER TABLE config DROP COLUMN strike_key;
ALTER TABLE config DROP COLUMN strike_endpoint;
