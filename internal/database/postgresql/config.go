package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func wrappedPublicKeysToBytes(npubs []cashu.WrappedPublicKey) ([][]byte, error) {
	if npubs == nil {
		return nil, nil
	}

	bytesList := make([][]byte, 0, len(npubs))
	for _, pubkey := range npubs {
		if pubkey.PublicKey == nil {
			return nil, fmt.Errorf("wrapped public key is nil")
		}
		serialized := pubkey.SerializeCompressed()
		serializedCopy := make([]byte, len(serialized))
		copy(serializedCopy, serialized)
		bytesList = append(bytesList, serializedCopy)
	}

	return bytesList, nil
}

func bytesToWrappedPublicKeys(rawPubkeys [][]byte) ([]cashu.WrappedPublicKey, error) {
	if rawPubkeys == nil {
		return nil, nil
	}

	parsed := make([]cashu.WrappedPublicKey, 0, len(rawPubkeys))
	for _, value := range rawPubkeys {
		var wrapped cashu.WrappedPublicKey
		if err := wrapped.Scan(value); err != nil {
			return nil, fmt.Errorf("wrapped.Scan(value): %w", err)
		}
		parsed = append(parsed, wrapped)
	}

	return parsed, nil
}

func (pql Postgresql) GetConfig(tx pgx.Tx) (utils.Config, error) {
	var config utils.Config

	err := tx.QueryRow(context.Background(), `SELECT
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
            peg_in_limit_sats,
            mint_require_auth,
            mint_auth_oicd_url,
            mint_auth_oicd_client_id,
            mint_auth_rate_limit_per_minute,
            mint_auth_max_blind_tokens,
            mint_auth_clear_auth_urls,
            mint_auth_blind_auth_urls,
            strike_key,
            strike_endpoint,
            icon_url,
            tos_url
         FROM config WHERE id = 1`).Scan(
		&config.NAME,
		&config.DESCRIPTION,
		&config.DESCRIPTION_LONG,
		&config.MOTD,
		&config.EMAIL,
		&config.NOSTR,
		&config.NETWORK,
		&config.MINT_LIGHTNING_BACKEND,
		&config.LND_GRPC_HOST,
		&config.LND_TLS_CERT,
		&config.LND_MACAROON,
		&config.MINT_LNBITS_ENDPOINT,
		&config.MINT_LNBITS_KEY,
		&config.CLN_GRPC_HOST,
		&config.CLN_CA_CERT,
		&config.CLN_CLIENT_CERT,
		&config.CLN_CLIENT_KEY,
		&config.CLN_MACAROON,
		&config.PEG_OUT_ONLY,
		&config.PEG_OUT_LIMIT_SATS,
		&config.PEG_IN_LIMIT_SATS,
		&config.MINT_REQUIRE_AUTH,
		&config.MINT_AUTH_OICD_URL,
		&config.MINT_AUTH_OICD_CLIENT_ID,
		&config.MINT_AUTH_RATE_LIMIT_PER_MINUTE,
		&config.MINT_AUTH_MAX_BLIND_TOKENS,
		&config.MINT_AUTH_CLEAR_AUTH_URLS,
		&config.MINT_AUTH_BLIND_AUTH_URLS,
		&config.STRIKE_KEY,
		&config.STRIKE_ENDPOINT,
		&config.IconUrl,
		&config.TosUrl,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return config, fmt.Errorf("could not find config in database: %w", err)
		}

		return config, fmt.Errorf("error checking for config: %w", err)
	}

	return config, nil
}

func (pql Postgresql) SetConfig(tx pgx.Tx, config utils.Config) error {
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
            peg_in_limit_sats,
            mint_require_auth,
            mint_auth_oicd_url,
            mint_auth_oicd_client_id,
            mint_auth_rate_limit_per_minute,
            mint_auth_max_blind_tokens,
            mint_auth_clear_auth_urls,
            mint_auth_blind_auth_urls,
			strike_key,
			strike_endpoint,
			icon_url,
			tos_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33)`

	for {
		tries += 1
		_, err := tx.Exec(context.Background(), stmt,
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
			config.MINT_REQUIRE_AUTH,
			config.MINT_AUTH_OICD_URL,
			config.MINT_AUTH_OICD_CLIENT_ID,
			config.MINT_AUTH_RATE_LIMIT_PER_MINUTE,
			config.MINT_AUTH_MAX_BLIND_TOKENS,
			config.MINT_AUTH_CLEAR_AUTH_URLS,
			config.MINT_AUTH_BLIND_AUTH_URLS,
			config.STRIKE_KEY,
			config.STRIKE_ENDPOINT,
			config.IconUrl,
			config.TosUrl,
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

func (pql Postgresql) UpdateConfig(tx pgx.Tx, config utils.Config) error {
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
            peg_in_limit_sats = $21,
            mint_require_auth = $22,
            mint_auth_oicd_url = $23,
            mint_auth_oicd_client_id = $24,
            mint_auth_rate_limit_per_minute = $25,
            mint_auth_max_blind_tokens = $26,
            mint_auth_clear_auth_urls = $27,
            mint_auth_blind_auth_urls = $28,
			strike_key = $29,
			strike_endpoint = $30,
			icon_url = $31,
			tos_url = $32
        WHERE id = 1`
		_, err := tx.Exec(context.Background(), stmt,
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
			config.MINT_REQUIRE_AUTH,
			config.MINT_AUTH_OICD_URL,
			config.MINT_AUTH_OICD_CLIENT_ID,
			config.MINT_AUTH_RATE_LIMIT_PER_MINUTE,
			config.MINT_AUTH_MAX_BLIND_TOKENS,
			config.MINT_AUTH_CLEAR_AUTH_URLS,
			config.MINT_AUTH_BLIND_AUTH_URLS,
			config.STRIKE_KEY,
			config.STRIKE_ENDPOINT,
			config.IconUrl,
			config.TosUrl,
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

func (pql Postgresql) GetNostrNotificationConfig(tx pgx.Tx) (*utils.NostrNotificationConfig, error) {
	var npubsRaw [][]byte
	var config utils.NostrNotificationConfig

	err := tx.QueryRow(context.Background(), `SELECT
            nostr_notification_npubs,
            nostr_notifications,
            nostr_notification_nip04_dm
         FROM nostr_notification_config WHERE id = 1`).Scan(
		&npubsRaw,
		&config.NOSTR_NOTIFICATIONS,
		&config.NOSTR_NOTIFICATION_NIP04_DM,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("error checking for nostr notification config: %w", err)
	}

	npubs, err := bytesToWrappedPublicKeys(npubsRaw)
	if err != nil {
		return nil, fmt.Errorf("bytesToWrappedPublicKeys(npubsRaw): %w", err)
	}
	config.NOSTR_NOTIFICATION_NPUBS = npubs

	return &config, nil
}

func (pql Postgresql) UpdateNostrNotificationConfig(tx pgx.Tx, config utils.NostrNotificationConfig) error {
	npubsBytes, err := wrappedPublicKeysToBytes(config.NOSTR_NOTIFICATION_NPUBS)
	if err != nil {
		return databaseError(fmt.Errorf("wrappedPublicKeysToBytes(config.NOSTR_NOTIFICATION_NPUBS): %w", err))
	}

	tries := 0
	for {
		tries += 1
		_, err = tx.Exec(context.Background(), `INSERT INTO nostr_notification_config (
			id,
			nostr_notification_npubs,
			nostr_notifications,
			nostr_notification_nip04_dm
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			nostr_notification_npubs = EXCLUDED.nostr_notification_npubs,
			nostr_notifications = EXCLUDED.nostr_notifications,
			nostr_notification_nip04_dm = EXCLUDED.nostr_notification_nip04_dm`,
			1,
			npubsBytes,
			config.NOSTR_NOTIFICATIONS,
			config.NOSTR_NOTIFICATION_NIP04_DM,
		)

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("could not update nostr notification config: %w", err))
		case err == nil:
			return nil
		}
	}
}
