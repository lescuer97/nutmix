-- +goose Up
CREATE TABLE user_auth (
	sub text NOT NULL,
	aud text,
	last_logged_in  int4 NOT NULL DEFAULT 0,
	CONSTRAINT user_auth_pk PRIMARY KEY (sub),
	CONSTRAINT user_auth_unique UNIQUE (sub)
);
CREATE INDEX IF NOT EXISTS idx_user_auth_sub ON user_auth (sub);

-- +goose Down
DROP TABLE IF EXISTS user_auth;
DROP INDEX idx_user_auth_sub;
