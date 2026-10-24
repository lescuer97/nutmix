package lightning

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
)

func TestGetFeatureBits(t *testing.T) {
	for _, bits := range [][]lnwire.FeatureBit{
		{},
		{lnwire.TLVOnionPayloadOptional},
		{lnwire.TLVOnionPayloadOptional, lnwire.PaymentAddrOptional},
	} {
		features := lnwire.NewFeatureVector(lnwire.NewRawFeatureVector(bits...), lnwire.Features)
		got := getFeatureBits(features)
		if len(got) != len(bits) {
			t.Fatalf("getFeatureBits(%v) returned %d entries, want %d", bits, len(got), len(bits))
		}
		seen := make(map[lnwire.FeatureBit]bool, len(bits))
		for _, bit := range got {
			feature := lnwire.FeatureBit(bit)
			if !features.IsSet(feature) || seen[feature] {
				t.Errorf("unexpected or duplicate feature bit: %v", bit)
			}
			seen[feature] = true
		}
	}
}

func TestUseMinimumFeeOnInvoice(t *testing.T) {
	chainParam := chaincfg.MainNetParams
	fakeWallet := FakeWallet{
		UnpurposeErrors: nil,
		Network:         chainParam,
		InvoiceFee:      2,
		NodeStatus:      ONLINE_STATUS,
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	invoiceString, err := CreateMockInvoice(cashu.NewAmount(cashu.Sat, 10000), "test", chainParam, expireTime)
	if err != nil {
		t.Fatalf(`CreateMockInvoice(10000, "test", chaincfg.MainNetParams,expireTime). %v`, err)
	}

	invoice, err := zpay32.Decode(invoiceString, &chainParam)

	if err != nil {
		t.Fatalf(`zpay32.Decode(invoiceString, &chainParam). %v`, err)
	}

	sat_amount := uint64(invoice.MilliSat.ToSatoshis())

	feeRes, err := fakeWallet.QueryFees(invoiceString, invoice, false, cashu.NewAmount(cashu.Sat, sat_amount))
	if err != nil {
		t.Fatalf(`fakeWallet.QueryFees(). %v`, err)
	}

	if feeRes.Fees.Amount != 100 {
		t.Errorf(`Fee is not being set to the correct value. %v`, feeRes.Fees.Amount)
	}
}

func TestUseFeeInvoice(t *testing.T) {
	chainParam := chaincfg.MainNetParams
	fakeWallet := FakeWallet{
		UnpurposeErrors: nil,
		Network:         chainParam,
		InvoiceFee:      150,
		NodeStatus:      ONLINE_STATUS,
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	invoiceString, err := CreateMockInvoice(cashu.NewAmount(cashu.Sat, 10000), "test", chainParam, expireTime)
	if err != nil {
		t.Fatalf(`CreateMockInvoice(10000, "test", chaincfg.MainNetParams,expireTime). %v`, err)
	}

	invoice, err := zpay32.Decode(invoiceString, &chainParam)

	if err != nil {
		t.Fatalf(`zpay32.Decode(invoiceString, &chainParam). %v`, err)
	}

	sat_amount := uint64(invoice.MilliSat.ToSatoshis())

	feeRes, err := fakeWallet.QueryFees(invoiceString, invoice, false, cashu.NewAmount(cashu.Sat, sat_amount))
	if err != nil {
		t.Fatalf(`fakeWallet.QueryFees(). %v`, err)
	}

	if feeRes.Fees.Amount != 150 {
		t.Errorf(`Fee is not being set to the correct value. %v`, feeRes.Fees)
	}
}
