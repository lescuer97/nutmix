package lightning

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

func TestUseMinimumFeeOnInvoice(t *testing.T) {
	chainParam := chaincfg.MainNetParams
	fakeWallet := FakeWallet{
		Network:    chainParam,
		InvoiceFee: 2,
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	invoiceString, err := CreateMockInvoice(10000, "test", chainParam, expireTime)
	if err != nil {
		t.Fatalf(`CreateMockInvoice(10000, "test", chaincfg.MainNetParams,expireTime). %v`, err)
	}

	invoice, err := zpay32.Decode(invoiceString, &chainParam)

	if err != nil {
		t.Fatalf(`zpay32.Decode(invoiceString, &chainParam). %v`, err)
	}

	sat_amount := uint64(invoice.MilliSat.ToSatoshis())

	fee, err := fakeWallet.QueryFees(invoiceString, invoice, false, sat_amount)
	if err != nil {
		t.Fatalf(`fakeWallet.QueryFees(). %v`, err)
	}

	if fee != 100 {

		t.Errorf(`Fee is not being set to the correct value. %v`, fee)
	}
}

func TestUseFeeInvoice(t *testing.T) {
	chainParam := chaincfg.MainNetParams
	fakeWallet := FakeWallet{
		Network:    chainParam,
		InvoiceFee: 150,
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	invoiceString, err := CreateMockInvoice(10000, "test", chainParam, expireTime)
	if err != nil {
		t.Fatalf(`CreateMockInvoice(10000, "test", chaincfg.MainNetParams,expireTime). %v`, err)
	}

	invoice, err := zpay32.Decode(invoiceString, &chainParam)

	if err != nil {
		t.Fatalf(`zpay32.Decode(invoiceString, &chainParam). %v`, err)
	}

	sat_amount := uint64(invoice.MilliSat.ToSatoshis())

	fee, err := fakeWallet.QueryFees(invoiceString, invoice, false, sat_amount)
	if err != nil {
		t.Fatalf(`fakeWallet.QueryFees(). %v`, err)
	}

	if fee != 150 {

		t.Errorf(`Fee is not being set to the correct value. %v`, fee)
	}
}
