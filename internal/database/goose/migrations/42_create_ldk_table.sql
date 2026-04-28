-- +goose Up
CREATE TABLE ldk (
    id INT NOT NULL,
    chain_source_type TEXT NOT NULL DEFAULT 'bitcoind',
    electrum_server_url TEXT NOT NULL DEFAULT '',
    rpc_address TEXT NOT NULL,
    rpc_username TEXT NOT NULL,
    rpc_password TEXT NOT NULL,
    rpc_port INT4 NOT NULL,
    config_directory TEXT NOT NULL,
    CONSTRAINT single_row CHECK (id = 1),
    CONSTRAINT ldk_id_pk PRIMARY KEY (id),
    CONSTRAINT ldk_chain_source_type_check CHECK (chain_source_type IN ('bitcoind', 'electrum')),
    CONSTRAINT ldk_rpc_port_range CHECK (rpc_port >= 0 AND rpc_port <= 65535)
);

-- +goose Down
DROP TABLE IF EXISTS ldk;
