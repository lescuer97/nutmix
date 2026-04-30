package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
)

func (pql Postgresql) GetLDKConfig(ctx context.Context) (database.LDKConfig, error) {
	var config database.LDKConfig

	err := pql.pool.QueryRow(ctx, `
		SELECT chain_source_type, electrum_server_url, esplora_server_url, rpc_address, rpc_username, rpc_password, rpc_port, config_directory
		FROM ldk
		WHERE id = 1
	`).Scan(&config.ChainSourceType, &config.ElectrumServerURL, &config.EsploraServerURL, &config.Rpc.Address, &config.Rpc.Username, &config.Rpc.Password, &config.Rpc.Port, &config.ConfigDirectory)
	if err != nil {
		if err == pgx.ErrNoRows {
			return database.LDKConfig{}, fmt.Errorf("ldk configuration not found: %w", err)
		}
		return database.LDKConfig{}, fmt.Errorf("pql.pool.QueryRow(get ldk config): %w", err)
	}

	return config, nil
}

func (pql Postgresql) SetLDKConfig(ctx context.Context, config database.LDKConfig) error {
	_, err := pql.pool.Exec(ctx, `
		INSERT INTO ldk (id, chain_source_type, electrum_server_url, esplora_server_url, rpc_address, rpc_username, rpc_password, rpc_port, config_directory)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			chain_source_type = EXCLUDED.chain_source_type,
			electrum_server_url = EXCLUDED.electrum_server_url,
			esplora_server_url = EXCLUDED.esplora_server_url,
			rpc_address = EXCLUDED.rpc_address,
			rpc_username = EXCLUDED.rpc_username,
			rpc_password = EXCLUDED.rpc_password,
			rpc_port = EXCLUDED.rpc_port,
			config_directory = EXCLUDED.config_directory
	`, config.ChainSourceType, config.ElectrumServerURL, config.EsploraServerURL, config.Rpc.Address, config.Rpc.Username, config.Rpc.Password, config.Rpc.Port, config.ConfigDirectory)
	if err != nil {
		return fmt.Errorf("pql.pool.Exec(set ldk config): %w", err)
	}

	return nil
}
