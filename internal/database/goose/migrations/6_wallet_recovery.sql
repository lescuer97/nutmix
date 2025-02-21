-- +goose Up
CREATE TABLE "recovery_signature" (
	amount int4 NULL,
    id text NOT NULL,
	"B_" text NOT NULL,
	"C_" text NOT NULL,
	created_at int8 NOT NULL
);


-- +goose Down
DROP TABLE "recovery_signature";
