package lightning

import (
	"context"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

// Strike preserves legacy configuration while rejecting all retired operations.
type Strike struct {
	Network chaincfg.Params
}

var _ LightningBackend = Strike{} //nolint:exhaustruct // Compile-time interface check only.

func (Strike) PayInvoice(cashu.MeltRequestDB, *zpay32.Invoice, cashu.Amount, bool, cashu.Amount) (PaymentResponse, error) {
	return PaymentResponse{}, ErrLNBackendEndOfLife
}

func (Strike) CheckPayed(string, *zpay32.Invoice, string) (PaymentStatus, string, cashu.Amount, error) {
	return UNKNOWN, "", cashu.Amount{}, ErrLNBackendEndOfLife
}

func (Strike) CheckReceived(cashu.MintRequestDB, *zpay32.Invoice) (PaymentStatus, string, error) {
	return UNKNOWN, "", ErrLNBackendEndOfLife
}

func (Strike) QueryFees(string, *zpay32.Invoice, bool, cashu.Amount) (FeesResponse, error) {
	return FeesResponse{}, ErrLNBackendEndOfLife
}

func (Strike) RequestInvoice(cashu.Amount, *string) (InvoiceResponse, error) {
	return InvoiceResponse{}, ErrLNBackendEndOfLife
}

func (Strike) WalletBalance() (cashu.Amount, error) {
	return cashu.Amount{}, ErrLNBackendEndOfLife
}

func (Strike) Status(context.Context) (NodeStatus, error) {
	return STOPPED_STATUS, ErrLNBackendEndOfLife
}

func (Strike) LightningType() Backend {
	return STRIKE
}

func (s Strike) GetNetwork() *chaincfg.Params {
	return &s.Network
}

func (Strike) ActiveMPP() bool {
	return false
}

func (Strike) VerifyUnitSupport(unit cashu.Unit) bool {
	return unit == cashu.Sat || unit == cashu.EUR
}

func (Strike) DescriptionSupport() bool {
	return true
}
