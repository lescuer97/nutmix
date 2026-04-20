package postgresql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

var likeWildcardEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLikePattern(value string) string {
	return likeWildcardEscaper.Replace(value)
}

func (pql Postgresql) GetMintRequestsByTime(ctx context.Context, since time.Time) ([]cashu.MintRequestDB, error) {
	sinceUnix := since.Unix()
	rows, err := pql.pool.Query(ctx, "SELECT quote, request, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description FROM mint_request WHERE seen_at >= $1", sinceUnix)
	if err != nil {
		return nil, fmt.Errorf("error checking for mint requests: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MintRequestDB])
}

func (pql Postgresql) GetMeltRequestsByTime(ctx context.Context, since time.Time) ([]cashu.MeltRequestDB, error) {
	sinceUnix := since.Unix()
	rows, err := pql.pool.Query(ctx, "SELECT quote, request, amount, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp, fee_paid, checking_id FROM melt_request WHERE seen_at >= $1", sinceUnix)
	if err != nil {
		return nil, fmt.Errorf("error checking for melt requests: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltRequestDB])
}

func (pql Postgresql) SearchLightningRequests(ctx context.Context, query string, since time.Time, limit int) ([]database.LightningActivityRow, error) {
	searchQuery := "%" + escapeLikePattern(query) + "%"
	sinceUnix := since.Unix()
	rows, err := pql.pool.Query(ctx, `
		SELECT id, type, request, state, unit, seen_at
		FROM (
			SELECT quote AS id, 'mint' AS type, request, state::text AS state, unit, seen_at
			FROM mint_request
			WHERE seen_at >= $1 AND (quote ILIKE $2 ESCAPE '\' OR request ILIKE $2 ESCAPE '\')
			UNION ALL
			SELECT quote AS id, 'melt' AS type, request, state::text AS state, unit, seen_at
			FROM melt_request
			WHERE seen_at >= $1 AND (quote ILIKE $2 ESCAPE '\' OR request ILIKE $2 ESCAPE '\')
		) lightning_activity
		ORDER BY seen_at DESC
		LIMIT $3
	`, sinceUnix, searchQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("error searching lightning requests: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[database.LightningActivityRow])
}
