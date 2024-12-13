package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (pql Postgresql) GetConfig() (utils.Config, error) {
	var config utils.Config

	rows, err := pql.pool.Query(context.Background(), `SELECT
            name,
            description,
            description_long,
            motd,
            email,
            nostr,
            network,
            mint_lightning_backend,
            lnd_grpc_host,
            lnd_tls_cert,
            lnd_macaroon,
            mint_lnbits_endpoint,
            mint_lnbits_key,
            cln_grpc_host,
            cln_ca_cert,
            cln_client_cert,
            cln_client_key,
            cln_macaroon,
            peg_out_only,
            peg_out_limit_sats,
            peg_in_limit_sats
         FROM config WHERE id = 1`)
	defer rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return config, fmt.Errorf("No rows found: %w", err)
		}

		return config, fmt.Errorf("Error checking for  seeds: %w", err)
	}

	config, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[utils.Config])

	if err != nil {
		return config, fmt.Errorf("pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[utils.Config]): %w", err)
	}

	return config, nil
}

func (pql Postgresql) SetConfig(config utils.Config) error {

	tries := 0
	stmt := `
        INSERT INTO config (
            id,
            name,
            description,
            description_long,
            motd,
            email,
            nostr,
            network,
            mint_lightning_backend,
            lnd_grpc_host,
            lnd_tls_cert,
            lnd_macaroon,
            mint_lnbits_endpoint,
            mint_lnbits_key,
            cln_grpc_host,
            cln_ca_cert,
            cln_client_cert,
            cln_client_key,
            cln_macaroon,
            peg_out_only,
            peg_out_limit_sats,
            peg_in_limit_sats
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`

	for {
		tries += 1
		_, err := pql.pool.Exec(context.Background(), stmt,
			1,
			config.NAME,
			config.DESCRIPTION,
			config.DESCRIPTION_LONG,
			config.MOTD,
			config.EMAIL,
			config.NOSTR,
			config.NETWORK,
			config.MINT_LIGHTNING_BACKEND,
			config.LND_GRPC_HOST,
			config.LND_TLS_CERT,
			config.LND_MACAROON,
			config.MINT_LNBITS_ENDPOINT,
			config.MINT_LNBITS_KEY,
			config.CLN_GRPC_HOST,
			config.CLN_CA_CERT,
			config.CLN_CLIENT_CERT,
			config.CLN_CLIENT_KEY,
			config.CLN_MACAROON,
			config.PEG_OUT_ONLY,
			config.PEG_OUT_LIMIT_SATS,
			config.PEG_IN_LIMIT_SATS,
		)

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("could not change config: %w", err))
		case err == nil:
			return nil
		}

	}
}

func (pql Postgresql) UpdateConfig(config utils.Config) error {

	tries := 0

	for {
		tries += 1

		stmt := `
        UPDATE config SET
            name = $1,
            description = $2,
            description_long = $3,
            motd = $4,
            email = $5,
            nostr = $6,
            network = $7,
            mint_lightning_backend = $8,
            lnd_grpc_host = $9,
            lnd_tls_cert = $10,
            lnd_macaroon = $11,
            mint_lnbits_endpoint = $12,
            mint_lnbits_key = $13,
            cln_grpc_host = $14,
            cln_ca_cert = $15,
            cln_client_cert = $16,
            cln_client_key = $17,
            cln_macaroon = $18,
            peg_out_only = $19,
            peg_out_limit_sats = $20,
            peg_in_limit_sats = $21
        WHERE id = 1`
		_, err := pql.pool.Exec(context.Background(), stmt,
			config.NAME,
			config.DESCRIPTION,
			config.DESCRIPTION_LONG,
			config.MOTD,
			config.EMAIL,
			config.NOSTR,
			config.NETWORK,
			config.MINT_LIGHTNING_BACKEND,
			config.LND_GRPC_HOST,
			config.LND_TLS_CERT,
			config.LND_MACAROON,
			config.MINT_LNBITS_ENDPOINT,
			config.MINT_LNBITS_KEY,
			config.CLN_GRPC_HOST,
			config.CLN_CA_CERT,
			config.CLN_CLIENT_CERT,
			config.CLN_CLIENT_KEY,
			config.CLN_MACAROON,
			config.PEG_OUT_ONLY,
			config.PEG_OUT_LIMIT_SATS,
			config.PEG_IN_LIMIT_SATS,
		)

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("could not change config: %w", err))
		case err == nil:
			return nil
		}

	}
}
