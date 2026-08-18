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

var _ LightningBackend = Strike{}

func (Strike) PayInvoice(cashu.MeltRequestDB, *zpay32.Invoice, cashu.Amount, bool, cashu.Amount) (PaymentResponse, error) {
	return PaymentResponse{}, LNBackendEndOfLife
}

func (Strike) CheckPayed(string, *zpay32.Invoice, string) (PaymentStatus, string, cashu.Amount, error) {
	return UNKNOWN, "", cashu.Amount{}, LNBackendEndOfLife
}

func (Strike) CheckReceived(cashu.MintRequestDB, *zpay32.Invoice) (PaymentStatus, string, error) {
	return UNKNOWN, "", LNBackendEndOfLife
}

func (Strike) QueryFees(string, *zpay32.Invoice, bool, cashu.Amount) (FeesResponse, error) {
	return FeesResponse{}, LNBackendEndOfLife
}

func (Strike) RequestInvoice(cashu.Amount, *string) (InvoiceResponse, error) {
	return InvoiceResponse{}, LNBackendEndOfLife
}

func (Strike) WalletBalance() (cashu.Amount, error) {
	return cashu.Amount{}, LNBackendEndOfLife
}

func (Strike) Status(context.Context) (NodeStatus, error) {
	return STOPPED_STATUS, LNBackendEndOfLife
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
