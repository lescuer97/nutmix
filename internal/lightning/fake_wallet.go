package lightning

import (
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type FakeWalletError int

const (
	NONE = 0

	FailPaymentPending = iota + 1
	FailPaymentFailed  = iota + 2
	FailPaymentUnknown = iota + 3

	FailQueryPending = iota + 4
	FailQueryFailed  = iota + 5
	FailQueryUnknown = iota + 6
)

type FakeWallet struct {
	Network         chaincfg.Params
	UnpurposeErrors []FakeWalletError
	InvoiceFee      uint64
}

const mock_preimage = "fakewalletpreimage"

func (f FakeWallet) PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	switch {
	case slices.Contains(f.UnpurposeErrors, FailPaymentUnknown):
		return PaymentResponse{
			Preimage:       "",
			PaymentRequest: "",
			PaymentState:   UNKNOWN,
			Rhash:          "",
			PaidFeeSat:     0,
		}, nil

	case slices.Contains(f.UnpurposeErrors, FailPaymentFailed):
		return PaymentResponse{
			Preimage:       "",
			PaymentRequest: "",
			PaymentState:   FAILED,
			Rhash:          "",
			PaidFeeSat:     0,
		}, nil
	case slices.Contains(f.UnpurposeErrors, FailPaymentPending):
		return PaymentResponse{
			Preimage:       "",
			PaymentRequest: "",
			PaymentState:   PENDING,
			Rhash:          "",
			PaidFeeSat:     0,
		}, nil
	}

	return PaymentResponse{
		Preimage:       mock_preimage,
		PaymentRequest: melt_quote.Request,
		PaymentState:   SETTLED,
		Rhash:          "",
		PaidFeeSat:     0,
		CheckingId:     melt_quote.CheckingId,
	}, nil
}

func (f FakeWallet) CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, uint64, error) {
	switch {
	case slices.Contains(f.UnpurposeErrors, FailQueryUnknown):
		return UNKNOWN, "", 0, nil
	case slices.Contains(f.UnpurposeErrors, FailQueryFailed):
		return FAILED, "", 0, nil
	case slices.Contains(f.UnpurposeErrors, FailQueryPending):
		return PENDING, "", 0, nil

	}

	return SETTLED, mock_preimage, uint64(10), nil
}

func (f FakeWallet) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	switch {
	case slices.Contains(f.UnpurposeErrors, FailQueryUnknown):
		return UNKNOWN, "", nil
	case slices.Contains(f.UnpurposeErrors, FailQueryFailed):
		return FAILED, "", nil
	case slices.Contains(f.UnpurposeErrors, FailQueryPending):
		return PENDING, "", nil

	}

	return SETTLED, mock_preimage, nil
}

func (f FakeWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error) {
	fee := GetFeeReserve(amount.Amount, f.InvoiceFee)
	hash := zpayInvoice.PaymentHash[:]
	feesResponse := FeesResponse{}
	feesResponse.Fees.Amount = fee
	feesResponse.AmountToSend.Amount = amount.Amount
	feesResponse.CheckingId = hex.EncodeToString(hash)

	return feesResponse, nil
}

func (f FakeWallet) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
	var response InvoiceResponse
	supported := f.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit). %w", cashu.ErrUnitNotSupported)
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)

	payReq, err := CreateMockInvoice(amount.Amount, "mock invoice", f.Network, expireTime)
	if err != nil {
		return response, fmt.Errorf(`CreateMockInvoice(amount, "mock invoice", f.Network, expireTime). %w`, err)
	}

	randUuid, err := uuid.NewRandom()

	if err != nil {
		return response, fmt.Errorf(`uuid.NewRandom() %w`, err)
	}

	return InvoiceResponse{
		PaymentRequest: payReq,
		Rhash:          randUuid.String(),
		CheckingId:     payReq,
	}, nil
}

func (f FakeWallet) WalletBalance() (uint64, error) {
	return 0, nil
}

func (f FakeWallet) LightningType() Backend {
	return FAKEWALLET
}

func (f FakeWallet) GetNetwork() *chaincfg.Params {
	return &f.Network
}

func (f FakeWallet) ActiveMPP() bool {
	return false
}
func (f FakeWallet) VerifyUnitSupport(unit cashu.Unit) bool {
	return true
}
