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

	feeRes, err := fakeWallet.QueryFees(invoiceString, invoice, false, cashu.Amount{Amount: sat_amount, Unit: cashu.Sat})
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

	feeRes, err := fakeWallet.QueryFees(invoiceString, invoice, false, cashu.Amount{Amount: sat_amount, Unit: cashu.Sat})
	if err != nil {
		t.Fatalf(`fakeWallet.QueryFees(). %v`, err)
	}

	if feeRes.Fees.Amount != 150 {

		t.Errorf(`Fee is not being set to the correct value. %v`, feeRes.Fees)
	}
}
