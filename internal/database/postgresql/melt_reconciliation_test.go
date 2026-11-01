//nolint:exhaustruct_v5 // Fixtures specify only reconciliation-relevant fields.
package postgresql

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestListMeltQuotesForReconciliation(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	if empty, err := db.ListMeltQuotesForReconciliation(ctx); err != nil || len(empty) != 0 {
		t.Fatalf("empty reconciliation list = %v, %v; want empty success", empty, err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()

	now := int64(1)
	quotes := []cashu.MeltRequestDB{
		{Quote: "reconcile-pending-empty", State: cashu.PENDING},
		{Quote: "reconcile-pending-proof", State: cashu.PENDING},
		{Quote: "reconcile-paid-pending", State: cashu.PAID, Melted: true},
		{Quote: "reconcile-paid-spent", State: cashu.PAID, Melted: true},
		{Quote: "reconcile-paid-no-proofs", State: cashu.PAID, Melted: true},
		{Quote: "reconcile-paid-unmelted", State: cashu.PAID},
	}
	for _, quote := range quotes {
		quote.Unit, quote.Request, quote.Expiry, quote.SeenAt = cashu.Sat.String(), "", now, now
		if err := db.SaveMeltRequest(tx, quote); err != nil {
			t.Fatalf("SaveMeltRequest(%s): %v", quote.Quote, err)
		}
	}
	proof := func(quote string, state cashu.ProofState, n byte) cashu.Proof {
		owner := quote
		return cashu.Proof{C: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n}).PubKey()}, Y: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n + 1}).PubKey()}, Id: "keyset", Secret: fmt.Sprintf("reconcile-proof-%d", n), Amount: 1, State: state, Quote: &owner}
	}
	proofs := []cashu.Proof{
		proof("reconcile-pending-proof", cashu.PROOF_PENDING, 31),
		proof("reconcile-paid-pending", cashu.PROOF_PENDING, 41),
		proof("reconcile-paid-pending", cashu.PROOF_PENDING, 51),
		proof("reconcile-paid-spent", cashu.PROOF_SPENT, 61),
		proof("reconcile-paid-unmelted", cashu.PROOF_PENDING, 71),
	}
	if err := db.SaveProof(ctx, tx, proofs); err != nil {
		t.Fatalf("SaveProof: %v", err)
	}
	if err := db.Commit(ctx, tx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := db.ListMeltQuotesForReconciliation(ctx)
	if err != nil {
		t.Fatalf("ListMeltQuotesForReconciliation: %v", err)
	}
	want := map[string]bool{"reconcile-pending-empty": true, "reconcile-pending-proof": true, "reconcile-paid-pending": true}
	if len(got) != len(want) {
		t.Fatalf("got %d quotes: %+v", len(got), got)
	}
	seen := make(map[string]bool, len(got))
	for _, quote := range got {
		if !want[quote.Quote] {
			t.Errorf("unexpected reconciliation quote %q", quote.Quote)
		}
		if seen[quote.Quote] {
			t.Errorf("duplicate reconciliation quote %q", quote.Quote)
		}
		seen[quote.Quote] = true
	}
	for quote := range want {
		if !seen[quote] {
			t.Errorf("missing reconciliation quote %q", quote)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.ListMeltQuotesForReconciliation(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reconciliation query error = %v", err)
	}
}
