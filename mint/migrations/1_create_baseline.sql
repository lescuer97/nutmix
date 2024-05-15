
-- +goose Up
CREATE TABLE "seeds" (
	seed bytea NOT NULL,
	active bool NOT NULL,
	unit text NOT NULL,
	id text NOT NULL,
	created_at int8 NOT NULL,
	CONSTRAINT seeds_pk PRIMARY KEY (id),
	CONSTRAINT seeds_unique UNIQUE (seed, id)
);

CREATE TABLE mint_request (
	quote text NOT NULL,
	request text NOT NULL,
	paid bool NOT NULL,
	expiry int4 NOT NULL,
	CONSTRAINT mint_request_pk PRIMARY KEY (quote)
);

create table melt_request (
	quote text NOT NULL,
	expiry int4 NOT NULL,
	fee_reserve int4 NOT NULL,
	request text NOT NULL,
	unit text NULL,
	amount int4 NOT NULL,
	paid bool NULL,
	CONSTRAINT melt_request_pk PRIMARY KEY (quote)
);

create table proofs (
	amount int4 NULL,
	id text NOT NULL,
	secret text NOT NULL,
	c text NOT NULL,
	CONSTRAINT proofs_seeds_fk FOREIGN KEY (id) REFERENCES seeds(id)
);


-- +goose Down
DROP TABLE IF EXISTS melt_request;
DROP TABLE IF EXISTS mint_request;
DROP TABLE IF EXISTS seeds;
DROP TABLE IF EXISTS proofs;
