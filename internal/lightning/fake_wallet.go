package lightning

import (
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
}

const mock_preimage = "fakewalletpreimage"

func (f FakeWallet) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error) {
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
		PaymentRequest: invoice,
		PaymentState:   SETTLED,
		Rhash:          "",
		PaidFeeSat:     0,
	}, nil
}

func (f FakeWallet) CheckPayed(quote string) (PaymentStatus, string, error) {
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

func (f FakeWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error) {
	return 0, nil
}

func (f FakeWallet) RequestInvoice(amount int64) (InvoiceResponse, error) {
	var response InvoiceResponse
	expireTime := cashu.ExpiryTimeMinUnit(15)

	payReq, err := CreateMockInvoice(amount, "mock invoice", f.Network, expireTime)
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
