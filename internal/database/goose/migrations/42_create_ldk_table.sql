-- +goose Up
CREATE TYPE ldk_chain_source_type AS ENUM ('bitcoind', 'electrum', 'esplora');

CREATE TABLE ldk (
    id INT NOT NULL,
    chain_source_type ldk_chain_source_type NOT NULL DEFAULT 'bitcoind',
    electrum_server_url TEXT NOT NULL DEFAULT '',
    esplora_server_url TEXT NOT NULL DEFAULT '',
    rpc_address TEXT NOT NULL,
    rpc_username TEXT NOT NULL,
    rpc_password TEXT NOT NULL,
    rpc_port INT4 NOT NULL,
    config_directory TEXT NOT NULL,
    CONSTRAINT single_row CHECK (id = 1),
    CONSTRAINT ldk_id_pk PRIMARY KEY (id),
    CONSTRAINT ldk_rpc_port_range CHECK (rpc_port >= 0 AND rpc_port <= 65535)
);

-- +goose Down
DROP TABLE IF EXISTS ldk;
DROP TYPE IF EXISTS ldk_chain_source_type;
