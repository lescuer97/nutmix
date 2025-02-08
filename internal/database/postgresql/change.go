package postgresql

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func (pql Postgresql) SaveMeltChange(change []cashu.BlindedMessage, quote string) error {
	entries := [][]any{}
	columns := []string{"B_", "id", "quote"}
	tableName := "melt_change_message"

	tries := 0

	for _, sig := range change {
		entries = append(entries, []any{sig.B_, sig.Id, quote})
	}

	for {
		tries += 1
		_, err := pql.pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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
func (pql Postgresql) GetMeltChangeByQuote(quote string) ([]cashu.MeltChange, error) {

	var meltChangeList []cashu.MeltChange

	rows, err := pql.pool.Query(context.Background(), `SELECT B_, id, quote FROM melt_change_message WHERE quote = ANY($1)`, quote)
	defer rows.Close()

	if err != nil {

		if err == pgx.ErrNoRows {
			return meltChangeList, nil
		}
	}

	meltChange, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltChange])

	if err != nil {
		if err == pgx.ErrNoRows {
			return meltChangeList, nil
		}
		return meltChangeList, fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err)
	}

	meltChangeList = meltChange

	return meltChangeList, nil
}
func (pql Postgresql) DeleteChangeByQuote(quote string) error {

	_, err := pql.pool.Exec(context.Background(), `DELETE FROM melt_change_message WHERE quote = $1`, quote)

	if err != nil {
		return databaseError(fmt.Errorf("pql.pool.Exec(context.Background(), `DELETE FROM melt_change_message WHERE quote = $1`, quote): %w", err))
	}

	return nil
}
