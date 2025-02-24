-- +goose Up
ALTER TABLE seeds 
DROP COLUMN seed,
DROP COLUMN encrypted;



-- +goose Down
ALTER TABLE seeds 
ADD seed bytea NOT NULL, 
ADD encrypted BOOL NOT NULL DEFAULT FALSE;
