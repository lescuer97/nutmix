//nolint:exhaustruct // Reconciliation tests specify only selection-relevant fields.
package mockdb

import (
	"context"
	"errors"
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestListMeltQuotesForReconciliation(t *testing.T) {
	paidQuote, otherQuote := "paid", "not-other"
	pendingQuote := "pending-inputs"
	db := &MockDB{
		MeltRequest: []cashu.MeltRequestDB{
			{Quote: "pending-empty", State: cashu.PENDING},
			{Quote: "pending-inputs", State: cashu.PENDING},
			{Quote: "paid", State: cashu.PAID, Melted: true},
			{Quote: "spent", State: cashu.PAID, Melted: true},
			{Quote: "no-proofs", State: cashu.PAID, Melted: true},
			{Quote: "unmelted", State: cashu.PAID, Melted: false},
			{Quote: "other", State: cashu.PAID, Melted: true},
		},
		Proofs: cashu.Proofs{
			{Quote: &pendingQuote, State: cashu.PROOF_PENDING},
			{Quote: &paidQuote, State: cashu.PROOF_PENDING},
			{Quote: &paidQuote, State: cashu.PROOF_PENDING},
			{Quote: &otherQuote, State: cashu.PROOF_PENDING},
			{Quote: strPtr("spent"), State: cashu.PROOF_SPENT},
			{Quote: strPtr("unmelted"), State: cashu.PROOF_PENDING},
		},
	}
	quotes, err := db.ListMeltQuotesForReconciliation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pending-empty", "pending-inputs", "paid"}
	if len(quotes) != len(want) {
		t.Fatalf("got %d quotes: %+v", len(quotes), quotes)
	}
	for i := range want {
		if quotes[i].Quote != want[i] {
			t.Errorf("quote[%d] = %q, want %q", i, quotes[i].Quote, want[i])
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.ListMeltQuotesForReconciliation(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query error = %v", err)
	}
	empty, err := (&MockDB{}).ListMeltQuotesForReconciliation(context.Background())
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty query = %v, %v; want empty success", empty, err)
	}
}

func strPtr(s string) *string { return &s }
