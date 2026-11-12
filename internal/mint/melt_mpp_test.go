package mint

import (
	"context"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
)

// mppWallet is the fake backend with multi-path payments on, as the LND and CLN backends report.
type mppWallet struct{ lightning.FakeWallet }

func (mppWallet) ActiveMPP() bool { return true }

// NUT-15: options.mpp.amount is in millisatoshis.
func TestMeltQuoteMppAmountIsMillisats(t *testing.T) {
	net := chaincfg.RegressionNetParams
	mint := &Mint{ //nolint:exhaustruct
		LightningBackend: mppWallet{lightning.FakeWallet{Network: net}}, //nolint:exhaustruct
		MintDB:           &mockdb.MockDB{},                              //nolint:exhaustruct
	}
	for _, c := range []struct {
		invoiceSat uint64
		mppMsat    uint64
		wantSat    uint64
		wantErr    string
	}{
		{64, 32_000, 32, ""},            // half of a 64-sat invoice
		{100_000, 50_000, 50, ""},       // 50 sat of a 100,000-sat invoice
		{64, 64_000, 64, ""},            // the whole invoice
		{64, 65_000, 0, "bigger than"},  // more than the invoice
		{64, 32_500, 0, "whole number"}, // not a whole number of sats
	} {
		inv, err := lightning.CreateMockInvoice(cashu.NewAmount(cashu.Sat, c.invoiceSat), "mpp", net, 3600)
		if err != nil {
			t.Fatal(err)
		}
		req := cashu.PostMeltQuoteBolt11Request{Request: inv, Unit: "sat"} //nolint:exhaustruct
		req.Options.Mpp = map[string]uint64{"amount": c.mppMsat}
		q, err := mint.CreateMeltQuote(context.Background(), req, Bolt11)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("invoice %d sat, mpp %d msat: err = %v, want %q", c.invoiceSat, c.mppMsat, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("invoice %d sat, mpp %d msat: %v", c.invoiceSat, c.mppMsat, err)
		}
		if q.Amount != c.wantSat || !q.Mpp {
			t.Errorf("invoice %d sat, mpp %d msat: quote %d sat (mpp=%v), want %d sat", c.invoiceSat, c.mppMsat, q.Amount, q.Mpp, c.wantSat)
		}
	}
}
