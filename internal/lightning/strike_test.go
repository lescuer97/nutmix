package lightning

import (
	"context"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestStrikeTombstone(t *testing.T) {
	strike := Strike{Network: chaincfg.RegressionNetParams}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "pay invoice", call: func() error {
			_, err := strike.PayInvoice(cashu.MeltRequestDB{}, nil, cashu.Amount{}, false, cashu.Amount{})
			return err
		}},
		{name: "check paid", call: func() error { _, _, _, err := strike.CheckPayed("", nil, ""); return err }},
		{name: "check received", call: func() error { _, _, err := strike.CheckReceived(cashu.MintRequestDB{}, nil); return err }},
		{name: "query fees", call: func() error { _, err := strike.QueryFees("", nil, false, cashu.Amount{}); return err }},
		{name: "request invoice", call: func() error { _, err := strike.RequestInvoice(cashu.Amount{}, nil); return err }},
		{name: "wallet balance", call: func() error { _, err := strike.WalletBalance(); return err }},
		{name: "status", call: func() error { _, err := strike.Status(context.Background()); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, LNBackendEndOfLife) {
				t.Fatalf("error = %v, want LNBackendEndOfLife", err)
			}
		})
	}

	status, _ := strike.Status(context.Background())
	if status != STOPPED_STATUS {
		t.Fatalf("Status() = %q, want %q", status, STOPPED_STATUS)
	}
	if strike.LightningType() != STRIKE || strike.GetNetwork().Name != chaincfg.RegressionNetParams.Name {
		t.Fatal("Strike metadata did not preserve its backend type and network")
	}
	if strike.ActiveMPP() || !strike.VerifyUnitSupport(cashu.Sat) || !strike.VerifyUnitSupport(cashu.EUR) || strike.VerifyUnitSupport(cashu.USD) || !strike.DescriptionSupport() {
		t.Fatal("Strike capability metadata changed unexpectedly")
	}
	if !IsBackendEndOfLife(strike) || IsBackendEndOfLife(FakeWallet{}) || IsBackendEndOfLife(nil) {
		t.Fatal("backend end-of-life detection returned an unexpected result")
	}
}
