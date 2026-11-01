package lightning

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

func TestLnbitsCheckPayedPropagatesCancellationToHTTP(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	invoice := new(zpay32.Invoice)
	invoice.PaymentHash = new([32]byte)
	go func() {
		_, _, _, err := (LnbitsWallet{Endpoint: server.URL, Key: "", Network: chaincfg.MainNetParams}).CheckPayed(ctx, "", invoice, "")
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("LNbits status request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CheckPayed() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("CheckPayed did not return after cancellation")
	}
}

func TestLnbitsPaymentState(t *testing.T) {
	tests := []struct {
		name   string
		status LNBitsPaymentStatus
		wanted PaymentStatus
	}{
		{
			name: "paid status settles payment",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    true,
				Pending: false,
			},
			wanted: SETTLED,
		},
		{
			name: "top level pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: true,
			},
			wanted: PENDING,
		},
		{
			name: "detail status pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "pending",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: false,
			},
			wanted: PENDING,
		},
		{
			name: "detail pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: true,
				},
				Paid:    false,
				Pending: false,
			},
			wanted: PENDING,
		},
		{
			name: "unpaid non pending fails payment",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: false,
			},
			wanted: FAILED,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := lnbitsPaymentState(test.status)
			if got != test.wanted {
				t.Fatalf("lnbitsPaymentState() = %v, want %v", got, test.wanted)
			}
		})
	}
}
