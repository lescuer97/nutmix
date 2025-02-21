-- +goose Up
CREATE TABLE IF NOT EXISTS "melt_change_message" (
	"B_" text NOT NULL,
	created_at int8 NOT NULL,
    quote text NOT NULL,
    id text NOT NULL
);


-- +goose Down
DROP TABLE "melt_change_message";
