//nolint:exhaustruct_v5 // Settlement fixtures set only relevant fields.
package postgresql

import (
	"context"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

func seedMeltSettlement(t *testing.T, quoteState cashu.ACTION_STATE, melted bool, fee uint64, preimage string, inputAmounts ...uint64) (Postgresql, context.Context, database.MeltSettlementCommand) {
	t.Helper()
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	quoteID := "settlement-quote"
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: quoteID, Request: "invoice", Unit: "sat", CheckingId: "checking", Amount: 5, State: cashu.PENDING, PaymentPreimage: preimage}); err != nil {
		t.Fatal(err)
	}
	inputs := make(cashu.Proofs, 0, len(inputAmounts))
	for i, amount := range inputAmounts {
		proof := cashu.Proof{
			C:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{byte(51 + i)}).PubKey()},
			Y:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{byte(61 + i)}).PubKey()},
			Id: "proof-keyset", Secret: "settlement-proof-" + string(rune('a'+i)), Amount: amount, State: cashu.PROOF_PENDING,
		}
		proof.Quote = &quoteID
		inputs = append(inputs, proof)
	}
	if err := db.SaveProof(ctx, tx, inputs); err != nil {
		t.Fatal(err)
	}
	first, second, third := lifecycleSignature(11), lifecycleSignature(12), lifecycleSignature(13)
	blanks := []cashu.BlindedMessage{first.GetBlindedMessage(), second.GetBlindedMessage(), third.GetBlindedMessage()}
	if err := db.ReserveMeltChangeOutputs(ctx, tx, blanks, quoteID); err != nil {
		t.Fatal(err)
	}
	var blankCount int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM blind_messages WHERE melt_quote_id=$1 AND amount IS NULL AND signature_id IS NULL", quoteID).Scan(&blankCount); err != nil || blankCount != len(blanks) {
		t.Fatalf("reserved blanks have known amounts: count=%d err=%v", blankCount, err)
	}
	if quoteState != cashu.PENDING || melted || fee != 0 {
		if err := db.ChangeMeltRequestState(ctx, tx, quoteID, quoteState, melted, fee); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return db, ctx, database.MeltSettlementCommand{
		QuoteID: quoteID, CheckingID: "checking", Unit: "sat", Inputs: inputs,
		Outputs: []cashu.SignedOutput{first, second}, LightningFee: 1, InputFee: 1, Preimage: "preimage",
	}
}

func TestCompleteMeltSettlementPersistsIdempotently(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	command.Outputs[1].Amount = 2
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.CompleteMeltSettlement(ctx, tx, command)
	if err != nil || result.AlreadyCompleted || result.Quote.State != cashu.PAID || !result.Quote.Melted || result.Quote.FeePaid != 1 || result.Quote.PaymentPreimage != "preimage" || len(result.Quote.Change) != 2 || result.Quote.Change[0].Amount != 1 || result.Quote.Change[1].Amount != 2 || len(result.Proofs) != 1 || result.Proofs[0].State != cashu.PROOF_SPENT {
		t.Fatalf("settlement=%+v err=%v", result, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	command.LightningFee, command.InputFee, command.Preimage = 99, 99, "override"
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := db.CompleteMeltSettlement(ctx, tx, command)
	if err != nil || !retry.AlreadyCompleted || retry.Quote.FeePaid != 1 || retry.Quote.PaymentPreimage != "preimage" || len(retry.Quote.Change) != 2 || retry.Quote.Change[0].Amount != 1 || retry.Quote.Change[1].Amount != 2 {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	read, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Rollback(ctx) }()
	for _, expected := range []struct {
		point  cashu.WrappedPublicKey
		amount uint64
	}{{lifecycleSignature(11).B_, 1}, {lifecycleSignature(12).B_, 2}} {
		recovered, err := db.GetRestoreSigsFromBlindedMessages(ctx, read, []cashu.WrappedPublicKey{expected.point})
		if err != nil || len(recovered) != 1 || recovered[0].MeltQuote != command.QuoteID || recovered[0].Amount != expected.amount {
			t.Fatalf("provenance recovery=%+v err=%v", recovered, err)
		}
	}
	var signatures, pending int
	if err := db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1)`, command.QuoteID).Scan(&signatures, &pending); err != nil || signatures != 2 || pending != 0 {
		t.Fatalf("signatures=%d pending=%d err=%v", signatures, pending, err)
	}
}

func TestCompleteMeltSettlementRejectsInvalidCommandsWithoutMutation(t *testing.T) {
	for _, tc := range []struct { //nolint:govet // Table favors name-first readability over packing.
		name   string
		mutate func(*database.MeltSettlementCommand)
	}{
		{"wrong owner", func(c *database.MeltSettlementCommand) { c.QuoteID = "other" }},
		{"checking id", func(c *database.MeltSettlementCommand) { c.CheckingID = "other" }},
		{"unit", func(c *database.MeltSettlementCommand) { c.Unit = "usd" }},
		{"missing proofs", func(c *database.MeltSettlementCommand) { c.Inputs = nil }},
		{"wrong proof", func(c *database.MeltSettlementCommand) { c.Inputs[0].Secret = "not-owned" }},
		{"over refund", func(c *database.MeltSettlementCommand) { c.Outputs[0].Amount = 4 }},
		{"wrong output order", func(c *database.MeltSettlementCommand) { c.Outputs[0], c.Outputs[1] = c.Outputs[1], c.Outputs[0] }},
		{"wrong keyset", func(c *database.MeltSettlementCommand) { c.Outputs[0].Id = "other" }},
		{"more outputs than blanks", func(c *database.MeltSettlementCommand) {
			c.Outputs = append(c.Outputs, lifecycleSignature(14), lifecycleSignature(15))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
			originalQuoteID := command.QuoteID
			tc.mutate(&command)
			tx, err := db.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.CompleteMeltSettlement(ctx, tx, command)
			if err == nil {
				t.Fatal("invalid command accepted")
			}
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			check, err := db.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = check.Rollback(ctx) }()
			quote, err := db.GetMeltRequestById(ctx, check, originalQuoteID)
			if err != nil || quote.State != cashu.PENDING {
				t.Fatalf("quote mutated: %+v %v", quote, err)
			}
			var state string
			if err := check.QueryRow(ctx, "SELECT state FROM proofs WHERE secret=$1", "settlement-proof-a").Scan(&state); err != nil || state != string(cashu.PROOF_PENDING) {
				t.Fatalf("proof state=%q err=%v", state, err)
			}
			var signatures, pending int
			if err := check.QueryRow(ctx, `SELECT (SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1)`, originalQuoteID).Scan(&signatures, &pending); err != nil || signatures != 0 || pending != 3 {
				t.Fatalf("invalid command mutated signatures=%d pending=%d err=%v", signatures, pending, err)
			}
		})
	}
}

func TestCompleteMeltSettlementPaidPreparationUsesDurablePayment(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PAID, true, 1, "durable-preimage", 10)
	command.LightningFee = 50
	command.Preimage = "untrusted-override"
	command.Outputs = command.Outputs[:1]
	command.Outputs[0].Amount = 3
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CompleteMeltSettlement(ctx, tx, command); !errors.Is(err, database.ErrMeltSettlementState) {
		t.Fatalf("mismatched durable payment metadata error=%v", err)
	}
	var state string
	var spent, signatures, pending int
	if err := tx.QueryRow(ctx, `SELECT state FROM melt_request WHERE quote=$1`, command.QuoteID).Scan(&state); err != nil || state != string(cashu.PAID) {
		t.Fatalf("rejected command changed quote state=%q err=%v", state, err)
	}
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM proofs WHERE state='SPENT'), (SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1)`, command.QuoteID).Scan(&spent, &signatures, &pending); err != nil || spent != 0 || signatures != 0 || pending != 3 {
		t.Fatalf("rejected command mutated data: spent=%d signatures=%d pending=%d err=%v", spent, signatures, pending, err)
	}
	command.LightningFee, command.Preimage = 1, "durable-preimage"
	result, err := db.CompleteMeltSettlement(ctx, tx, command)
	if err != nil || result.AlreadyCompleted || result.Quote.PaymentPreimage != "durable-preimage" || result.Quote.FeePaid != 1 || result.Proofs[0].State != cashu.PROOF_SPENT {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(result.Quote.Change) != 1 {
		t.Fatalf("prepared change=%v", result.Quote.Change)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteMeltSettlementRejectsPaidUnmeltedQuote(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PAID, false, 1, "durable-preimage", 10)
	command.Outputs = command.Outputs[:1]
	command.Outputs[0].Amount = 3
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CompleteMeltSettlement(ctx, tx, command)
	if !errors.Is(err, database.ErrMeltSettlementState) {
		t.Fatalf("PAID but unmelted quote error=%v", err)
	}
	_ = tx.Rollback(ctx)
}

func TestCompleteMeltSettlementZeroRefundReleasesBlanks(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 7)
	command.Outputs = nil // Input 7 exactly covers amount 5 and fees 1+1.
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.CompleteMeltSettlement(ctx, tx, command)
	if err != nil || result.Quote.State != cashu.PAID || len(result.Quote.Change) != 0 {
		t.Fatalf("zero-refund settlement=%+v err=%v", result, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var blanks, signatures int
	if err := db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blind_messages WHERE melt_quote_id=$1 AND state='PENDING'), (SELECT count(*) FROM blind_signatures)`, command.QuoteID).Scan(&blanks, &signatures); err != nil || blanks != 0 || signatures != 0 {
		t.Fatalf("zero-refund blanks=%d signatures=%d err=%v", blanks, signatures, err)
	}
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: "reuse-quote", Request: "invoice", Unit: "sat", CheckingId: "reuse-checking", Amount: 1, State: cashu.PENDING}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReserveMeltChangeOutputs(ctx, tx, []cashu.BlindedMessage{lifecycleSignature(13).GetBlindedMessage()}, "reuse-quote"); err != nil {
		t.Fatalf("unused blank was not reusable: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteMeltSettlementAllowsRefundBelowAvailableChange(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	command.Outputs = command.Outputs[:1]
	command.Outputs[0].Amount = 2 // Refund is 3; unused refund and blanks are released.
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.CompleteMeltSettlement(ctx, tx, command)
	if err != nil || len(result.Quote.Change) != 1 || result.Quote.Change[0].Amount != 2 {
		t.Fatalf("partial refund=%+v err=%v", result, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1", command.QuoteID).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("partial refund left pending blanks=%d err=%v", pending, err)
	}
}

func TestMeltChangeReservationConflictsWithGenericWriter(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.InsertSignedOutputs(ctx, tx, []cashu.SignedOutput{command.Outputs[0]}); !errors.Is(err, cashu.ErrOutputPending) {
		t.Fatalf("generic writer bypassed melt ownership: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var signatures, pending int
	if err := db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1)`, command.QuoteID).Scan(&signatures, &pending); err != nil || signatures != 0 || pending != 3 {
		t.Fatalf("generic collision changed rows: signatures=%d pending=%d err=%v", signatures, pending, err)
	}
}

func TestCompleteMeltSettlementRejectsCheckedOverflowAndMixedProofs(t *testing.T) {
	for _, fee := range []struct {
		name      string
		lightning uint64
		input     uint64
	}{
		{"lightning fee", ^uint64(0), 1},
		{"input fee", 1, ^uint64(0)},
	} {
		t.Run(fee.name, func(t *testing.T) {
			db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
			command.LightningFee, command.InputFee = fee.lightning, fee.input
			tx, err := db.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.CompleteMeltSettlement(ctx, tx, command)
			if !errors.Is(err, cashu.ErrAmountOverflow) {
				t.Fatalf("overflow error=%v", err)
			}
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
		})
	}
	t.Run("mixed proof states", func(t *testing.T) {
		db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 6, 4)
		if _, err := db.pool.Exec(ctx, "UPDATE proofs SET state='SPENT' WHERE secret=$1", command.Inputs[0].Secret); err != nil {
			t.Fatal(err)
		}
		tx, err := db.GetTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.CompleteMeltSettlement(ctx, tx, command)
		if !errors.Is(err, database.ErrMeltSettlementState) {
			t.Fatalf("mixed proof state error=%v", err)
		}
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
	})
}

func TestCompleteMeltSettlementRejectsOutputAmountOverflow(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	command.Outputs[0].Amount, command.Outputs[1].Amount = ^uint64(0), 1
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CompleteMeltSettlement(ctx, tx, command)
	if !errors.Is(err, cashu.ErrAmountOverflow) {
		t.Fatalf("signed-output overflow error=%v", err)
	}
	if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
}

func TestCompleteMeltSettlementConcurrentLockAndRetry(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	command.Outputs = command.Outputs[:1]
	command.Outputs[0].Amount = 3
	winner, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CompleteMeltSettlement(ctx, winner, command); err != nil {
		t.Fatal(err)
	}
	loser, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CompleteMeltSettlement(ctx, loser, command)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("concurrent loser error=%v", err)
	}
	if err := loser.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := winner.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	retry, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.CompleteMeltSettlement(ctx, retry, command)
	if err != nil || !result.AlreadyCompleted {
		t.Fatalf("post-commit retry=%+v err=%v", result, err)
	}
	if err := retry.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var signatures int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM blind_signatures").Scan(&signatures); err != nil || signatures != 1 {
		t.Fatalf("duplicate signatures=%d err=%v", signatures, err)
	}
}

func TestCompleteMeltSettlementRollbackPreservesPgError(t *testing.T) {
	db, ctx, command := seedMeltSettlement(t, cashu.PENDING, false, 0, "", 10)
	if _, err := db.pool.Exec(ctx, `CREATE FUNCTION fail_settlement_quote_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'settlement trigger failure' USING ERRCODE='23514', CONSTRAINT='test_settlement_failure'; END $$;
		CREATE TRIGGER fail_settlement_quote_update BEFORE UPDATE ON melt_request FOR EACH ROW EXECUTE FUNCTION fail_settlement_quote_update()`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CompleteMeltSettlement(ctx, tx, command)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "test_settlement_failure" {
		t.Fatalf("expected original trigger PgError, got %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	check, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = check.Rollback(ctx) }()
	quote, err := db.GetMeltRequestById(ctx, check, command.QuoteID)
	if err != nil || quote.State != cashu.PENDING {
		t.Fatalf("quote rollback=%+v err=%v", quote, err)
	}
	var spent, signatures, pending int
	if err := check.QueryRow(ctx, `SELECT (SELECT count(*) FROM proofs WHERE state='SPENT'), (SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state='PENDING' AND melt_quote_id=$1)`, command.QuoteID).Scan(&spent, &signatures, &pending); err != nil || spent != 0 || signatures != 0 || pending != 3 {
		t.Fatalf("rollback leaked writes: spent=%d signatures=%d pending=%d err=%v", spent, signatures, pending, err)
	}
}
