-- +goose Up
ALTER TABLE melt_request ADD mpp bool NOT NULL DEFAULT false;



-- +goose Down
ALTER TABLE melt_request DROP COLUMN mpp;
