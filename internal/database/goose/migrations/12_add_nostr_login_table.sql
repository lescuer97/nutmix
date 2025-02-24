-- +goose Up
CREATE TABLE "nostr_login" (
	nonce text NOT NULL,
	expiry int8 NOT NULL,
	activated bool NOT NULL,
	CONSTRAINT nostr_login_pk PRIMARY KEY (nonce),
	CONSTRAINT nostr_login_unique UNIQUE (nonce)
);



-- +goose Down
DROP TABLE IF EXISTS nostr_login;
