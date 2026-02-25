package ldk

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/zpay32"
)

func mustDecodeMockInvoice(t *testing.T) *zpay32.Invoice {
	t.Helper()

	invoiceString, err := lightning.CreateMockInvoice(cashu.NewAmount(cashu.Sat, 1000), "test", chaincfg.RegressionNetParams, 3600)
	if err != nil {
		t.Fatalf("lightning.CreateMockInvoice(...): %v", err)
	}

	invoice, err := zpay32.Decode(invoiceString, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("zpay32.Decode(...): %v", err)
	}

	return invoice
}

func TestFilterPaymentsByType(t *testing.T) {
	payments := []ldk_node.PaymentDetails{
		{Id: "out-1", Direction: ldk_node.PaymentDirectionOutbound},
		{Id: "in-1", Direction: ldk_node.PaymentDirectionInbound},
		{Id: "out-2", Direction: ldk_node.PaymentDirectionOutbound},
	}

	tests := []struct {
		name        string
		paymentType PaymentType
		wantIDs     []string
		wantErr     string
	}{
		{name: "all", paymentType: All, wantIDs: []string{"out-1", "in-1", "out-2"}},
		{name: "incoming", paymentType: Incoming, wantIDs: []string{"in-1"}},
		{name: "outgoing", paymentType: Outgoing, wantIDs: []string{"out-1", "out-2"}},
		{name: "unknown", paymentType: PaymentType(99), wantErr: "unknown payment type: 99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterPaymentsByType(payments, tt.paymentType)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("filterPaymentsByType(...) error = %v, want %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("filterPaymentsByType(...) error = %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("filterPaymentsByType(...) len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, wantID := range tt.wantIDs {
				if got[i].Id != wantID {
					t.Fatalf("filterPaymentsByType(...)[%d].Id = %q, want %q", i, got[i].Id, wantID)
				}
			}
		})
	}
}

func TestFindPaymentDetailsPrefersCheckingID(t *testing.T) {
	hash := "invoice-hash"
	fee := uint64(250)
	newerHashMatch := ldk_node.PaymentDetails{
		Id:                    "newer-hash-match",
		Direction:             ldk_node.PaymentDirectionOutbound,
		LatestUpdateTimestamp: 20,
		Kind: ldk_node.PaymentKindBolt11{
			Hash: hash,
		},
	}
	exactByID := &ldk_node.PaymentDetails{
		Id:                    "exact-id",
		Direction:             ldk_node.PaymentDirectionOutbound,
		LatestUpdateTimestamp: 10,
		Status:                ldk_node.PaymentStatusSucceeded,
		FeePaidMsat:           &fee,
		Kind: ldk_node.PaymentKindBolt11{
			Hash: hash,
		},
	}

	got := findPaymentDetails([]ldk_node.PaymentDetails{newerHashMatch}, exactByID, ldk_node.PaymentDirectionOutbound, hash)
	if got == nil {
		t.Fatal("findPaymentDetails(...) returned nil")
	}
	if got.Id != "exact-id" {
		t.Fatalf("findPaymentDetails(...).Id = %q, want %q", got.Id, "exact-id")
	}
}

func TestFindPaymentDetailsDoesNotFallbackToLatestWhenHashMissing(t *testing.T) {
	payments := []ldk_node.PaymentDetails{
		{
			Id:                    "different-hash",
			Direction:             ldk_node.PaymentDirectionOutbound,
			LatestUpdateTimestamp: 99,
			Kind:                  ldk_node.PaymentKindBolt11{Hash: "different"},
		},
	}

	got := findPaymentDetails(payments, nil, ldk_node.PaymentDirectionOutbound, "wanted")
	if got != nil {
		t.Fatalf("findPaymentDetails(...) = %+v, want nil", got)
	}
}

func TestPaymentStatusFromDetailsNilFeePaidMsat(t *testing.T) {
	preimage := "preimage"
	status, gotPreimage, fee, err := paymentStatusFromDetails(&ldk_node.PaymentDetails{
		Status: ldk_node.PaymentStatusSucceeded,
		Kind: ldk_node.PaymentKindBolt11{
			Preimage: &preimage,
		},
	})
	if err != nil {
		t.Fatalf("paymentStatusFromDetails(... ) error = %v", err)
	}

	if status != SETTLED {
		t.Fatalf("paymentStatusFromDetails(... ) status = %v, want %v", status, SETTLED)
	}
	if gotPreimage != preimage {
		t.Fatalf("paymentStatusFromDetails(... ) preimage = %q, want %q", gotPreimage, preimage)
	}
	if fee.Amount != 0 || fee.Unit != cashu.Msat {
		t.Fatalf("paymentStatusFromDetails(... ) fee = %+v, want zero msat", fee)
	}
}

func TestPaymentStatusFromDetailsRejectsUnknownStatus(t *testing.T) {
	_, _, _, err := paymentStatusFromDetails(&ldk_node.PaymentDetails{
		Status: ldk_node.PaymentStatus(999),
		Kind: ldk_node.PaymentKindBolt11{
			Hash: "invoice-hash",
		},
	})
	if err == nil {
		t.Fatal("expected unknown status error")
	}
}

func TestCheckPayedRejectsNilInvoice(t *testing.T) {
	backend := &LDK{}
	status, _, _, err := backend.CheckPayed("", nil, "")
	if err == nil {
		t.Fatal("expected nil invoice error")
	}
	if status != UNKNOWN {
		t.Fatalf("status = %v, want %v", status, UNKNOWN)
	}
}

func TestCheckReceivedRejectsNilInvoice(t *testing.T) {
	backend := &LDK{}
	status, _, err := backend.CheckReceived(cashu.MintRequestDB{}, nil)
	if err == nil {
		t.Fatal("expected nil invoice error")
	}
	if status != UNKNOWN {
		t.Fatalf("status = %v, want %v", status, UNKNOWN)
	}
}

func TestCheckPayedPropagatesGetNodeError(t *testing.T) {
	backend := &LDK{}
	status, _, _, err := backend.CheckPayed("", mustDecodeMockInvoice(t), "")
	if err == nil {
		t.Fatal("expected getNode error")
	}
	if status != UNKNOWN {
		t.Fatalf("status = %v, want %v", status, UNKNOWN)
	}
}

func TestCheckReceivedPropagatesGetNodeError(t *testing.T) {
	backend := &LDK{}
	status, _, err := backend.CheckReceived(cashu.MintRequestDB{}, mustDecodeMockInvoice(t))
	if err == nil {
		t.Fatal("expected getNode error")
	}
	if status != UNKNOWN {
		t.Fatalf("status = %v, want %v", status, UNKNOWN)
	}
}

func TestLDKFinishRunClearsLifecycleState(t *testing.T) {
	doneCh := make(chan struct{})
	backend := &LDK{
		started: true,
		doneCh:  doneCh,
	}

	backend.finishRun(doneCh)

	if backend.started {
		t.Fatal("expected started to be false")
	}
	if backend.doneCh != nil {
		t.Fatal("expected doneCh to be cleared")
	}
}

func TestLDKStopIsSafeWhenNotStarted(t *testing.T) {
	backend := &LDK{}
	if err := backend.Stop(); err != nil {
		t.Fatalf("backend.Stop(): %v", err)
	}
}
