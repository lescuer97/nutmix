package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func (pql Postgresql) SaveMeltChange(tx pgx.Tx, change []cashu.BlindedMessage, quote string) error {
	entries := [][]any{}
	columns := []string{`B_`, "created_at", "id", "quote"}
	tableName := "melt_change_message"

	tries := 0

	now := time.Now().Unix()
	for _, sig := range change {
		entries = append(entries, []any{sig.B_, now, sig.Id, quote})
	}

	for {
		tries += 1
		_, err := tx.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("inserting to DB: %w", err))
		case err == nil:
			return nil
		}

	}
}

func (pql Postgresql) GetMeltChangeByQuote(tx pgx.Tx, quote string) ([]cashu.MeltChange, error) {

	meltChangeList := make([]cashu.MeltChange, 0)

	rows, err := tx.Query(context.Background(), `SELECT "B_", id, quote, created_at FROM melt_change_message WHERE quote = $1 FOR UPDATE NOWAIT`, quote)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return meltChangeList, nil
		}
	}
	defer rows.Close()

	meltChange, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltChange])

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return meltChangeList, nil
		}
		return meltChangeList, fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err)
	}

	meltChangeList = meltChange

	return meltChangeList, nil
}
func (pql Postgresql) DeleteChangeByQuote(tx pgx.Tx, quote string) error {

	_, err := tx.Exec(context.Background(), `DELETE FROM melt_change_message WHERE quote = $1`, quote)

	if err != nil {
		return databaseError(fmt.Errorf("pql.pool.Exec(context.Background(), `DELETE FROM melt_change_message WHERE quote = $1`, quote): %w", err))
	}

	return nil
}
