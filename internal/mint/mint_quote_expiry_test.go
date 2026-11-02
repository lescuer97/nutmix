package mint

import (
	"context"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
)

type mintQuoteExpiryBackend struct {
	lightning.LightningBackend
	invoice string
}

func (b mintQuoteExpiryBackend) RequestInvoice(cashu.Amount, *string) (lightning.InvoiceResponse, error) {
	return lightning.InvoiceResponse{PaymentRequest: b.invoice, CheckingId: "", Rhash: ""}, nil
}

func (mintQuoteExpiryBackend) GetNetwork() *chaincfg.Params { return &chaincfg.RegressionNetParams }

func TestMintQuoteUsesInvoiceExpiry(t *testing.T) {
	const invoiceTimestamp = int64(1_700_000_000)
	for _, tc := range []struct {
		name       string
		invoiceTTL time.Duration
	}{
		{name: "explicit expiry", invoiceTTL: 45 * time.Minute},
		{name: "decoder default expiry", invoiceTTL: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invoice := mintQuoteExpiryInvoice(t, time.Unix(invoiceTimestamp, 0), tc.invoiceTTL)
			wantExpiry := invoiceTimestamp + int64((time.Hour).Seconds())
			if tc.invoiceTTL != 0 {
				wantExpiry = invoiceTimestamp + int64(tc.invoiceTTL.Seconds())
			}

			db := &mockdb.MockDB{} //nolint:exhaustruct // only MintRequest is inspected
			mint := Mint{          //nolint:exhaustruct // quote creation only uses the backend and database
				LightningBackend: mintQuoteExpiryBackend{LightningBackend: nil, invoice: invoice},
				MintDB:           db,
			} //nolint:exhaustruct // quote creation only uses the backend and database
			response, err := mint.createBolt11MintQuote(context.Background(), cashu.PostMintQuoteBolt11Request{
				Amount:      125,
				Unit:        cashu.Sat.String(),
				Description: nil,
				Pubkey:      cashu.WrappedPublicKey{}, //nolint:exhaustruct // nil key is the baseline quote behavior
			}, cashu.Sat) //nolint:exhaustruct // only quote amount/unit affect this path
			if err != nil {
				t.Fatalf("createBolt11MintQuote: %v", err)
			}
			if response.Expiry != wantExpiry {
				t.Errorf("response expiry = %d, want %d", response.Expiry, wantExpiry)
			}
			if len(db.MintRequest) != 1 || db.MintRequest[0].Expiry != wantExpiry {
				t.Fatalf("saved mint request expiry = %+v, want %d", db.MintRequest, wantExpiry)
			}
			if db.MintRequest[0].Amount == nil || *db.MintRequest[0].Amount != 125 {
				t.Errorf("saved amount = %v, want 125", db.MintRequest[0].Amount)
			}
		})
	}
}

func TestMintQuoteRejectsInvalidInvoice(t *testing.T) {
	db := &mockdb.MockDB{} //nolint:exhaustruct // only MintRequest is inspected
	mint := Mint{          //nolint:exhaustruct // quote creation only uses the backend and database
		LightningBackend: mintQuoteExpiryBackend{LightningBackend: nil, invoice: "not a bolt11 invoice"},
		MintDB:           db,
	} //nolint:exhaustruct // quote creation only uses the backend and database

	_, err := mint.createBolt11MintQuote(context.Background(), cashu.PostMintQuoteBolt11Request{
		Amount:      125,
		Unit:        cashu.Sat.String(),
		Description: nil,
		Pubkey:      cashu.WrappedPublicKey{}, //nolint:exhaustruct // nil key is the baseline quote behavior
	}, cashu.Sat) //nolint:exhaustruct // only quote amount/unit affect this path
	if err == nil {
		t.Fatal("createBolt11MintQuote error = nil, want invalid invoice error")
	}
	if len(db.MintRequest) != 0 {
		t.Fatalf("saved %d mint requests for invalid invoice, want none", len(db.MintRequest))
	}
}

func mintQuoteExpiryInvoice(t *testing.T, timestamp time.Time, ttl time.Duration) string {
	t.Helper()
	var paymentHash [32]byte
	paymentHash[0] = 1
	options := []func(*zpay32.Invoice){zpay32.Description("mint quote expiry test"), zpay32.Amount(lnwire.MilliSatoshi(125_000))}
	if ttl != 0 {
		options = append(options, zpay32.Expiry(ttl))
	}
	invoice, err := zpay32.NewInvoice(&chaincfg.RegressionNetParams, paymentHash, timestamp, options...)
	if err != nil {
		t.Fatalf("zpay32.NewInvoice: %v", err)
	}
	privateKey := secp256k1.PrivKeyFromBytes([]byte{1})
	encoded, err := invoice.Encode(zpay32.MessageSigner{SignCompact: func(message []byte) ([]byte, error) {
		return ecdsa.SignCompact(privateKey, message, true), nil
	}})
	if err != nil {
		t.Fatalf("invoice.Encode: %v", err)
	}
	return encoded
}
