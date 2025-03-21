-- +goose Up
CREATE TABLE user_auth (
	sub text NOT NULL,
	aud text,
	last_logged_in  int4 NOT NULL DEFAULT 0,
	CONSTRAINT user_auth_pk PRIMARY KEY (sub),
	CONSTRAINT user_auth_unique UNIQUE (sub)
);



-- +goose Down
DROP TABLE IF EXISTS user_auth;
