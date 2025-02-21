-- +goose Up
ALTER TABLE config ADD strike_key text;
UPDATE config SET strike_key = "" WHERE strike_key IS NULL;



-- +goose Down
ALTER TABLE config DROP COLUMN strike_key;
