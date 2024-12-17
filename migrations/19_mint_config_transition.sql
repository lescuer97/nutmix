-- +goose Up
CREATE TABLE config (
    id INT DEFAULT 1,
    name TEXT,
    description TEXT,
    description_long TEXT,
    motd TEXT,
    email TEXT,
    nostr TEXT,
    network TEXT,
    mint_lightning_backend TEXT,
    lnd_grpc_host TEXT,
    lnd_tls_cert TEXT,
    lnd_macaroon TEXT,
    mint_lnbits_endpoint TEXT,
    mint_lnbits_key TEXT,
    cln_grpc_host TEXT,
    cln_ca_cert TEXT,
    cln_client_cert TEXT,
    cln_client_key TEXT,
    cln_macaroon TEXT,

    peg_out_only BOOLEAN DEFAULT FALSE,
    peg_out_limit_sats INTEGER,
    peg_in_limit_sats INTEGER,

    CONSTRAINT single_row CHECK (id = 1),
	CONSTRAINT config_id_pk PRIMARY KEY (id)
);


-- +goose Down
DROP TABLE IF EXISTS config;
