package mint

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

type transactionCleanupDB struct { //nolint:govet // Test double field order is not performance critical.
	database.MintDB
	meltRequest cashu.MeltRequestDB
	proofs      cashu.Proofs
	restoreErr  error
	rollbacks   int
}

func (db *transactionCleanupDB) GetTx(context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (db *transactionCleanupDB) Rollback(context.Context, pgx.Tx) error {
	db.rollbacks++
	return nil
}

func (db *transactionCleanupDB) GetMeltRequestById(context.Context, pgx.Tx, string) (cashu.MeltRequestDB, error) {
	return db.meltRequest, nil
}

func (db *transactionCleanupDB) GetRestoreSigsFromBlindedMessages(context.Context, pgx.Tx, []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error) {
	return nil, db.restoreErr
}

func (db *transactionCleanupDB) GetProofsFromSecretCurve(context.Context, pgx.Tx, []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	return db.proofs, nil
}

func TestTransactionCleanupOnEarlyReturns(t *testing.T) {
	t.Run("paid melt quote", func(t *testing.T) {
		db := &transactionCleanupDB{meltRequest: cashu.MeltRequestDB{Quote: "quote", State: cashu.PAID}} //nolint:exhaustruct_v5 // Only quote state matters.
		mint := Mint{MintDB: db}                                                                         //nolint:exhaustruct_v5 // Only the database is used.

		_, err := CheckMeltRequest(context.Background(), &mint, "quote")
		if err != nil {
			t.Fatalf("CheckMeltRequest() error = %v", err)
		}
		if db.rollbacks != 1 {
			t.Fatalf("rollback count = %d, want 1", db.rollbacks)
		}
	})

	t.Run("restore query error", func(t *testing.T) {
		db := &transactionCleanupDB{restoreErr: errors.New("restore failed")} //nolint:exhaustruct_v5 // Only the injected error matters.
		mint := Mint{MintDB: db}                                              //nolint:exhaustruct_v5 // Only the database is used.

		_, err := mint.Restore(context.Background(), cashu.PostRestoreRequest{}) //nolint:exhaustruct_v5 // Empty outputs exercise the query error path.
		if err == nil {
			t.Fatal("Restore() error = nil, want query error")
		}
		if db.rollbacks != 1 {
			t.Fatalf("rollback count = %d, want 1", db.rollbacks)
		}
	})

	t.Run("used auth proof", func(t *testing.T) {
		db := &transactionCleanupDB{proofs: cashu.Proofs{{}}} //nolint:exhaustruct_v5 // Only proof presence matters.
		mint := Mint{MintDB: db}                              //nolint:exhaustruct_v5 // Only the database is used.

		err := mint.VerifyAuthBlindToken(context.Background(), cashu.AuthProof{Secret: "used"}) //nolint:exhaustruct_v5 // Only the secret is used.
		if err == nil || err.Error() != "authProof already used" {
			t.Fatalf("VerifyAuthBlindToken() error = %v, want authProof already used", err)
		}
		if db.rollbacks != 1 {
			t.Fatalf("rollback count = %d, want 1", db.rollbacks)
		}
	})
}
