package lightning

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type Backend uint

const LNDGRPC Backend = iota + 1
const LNBITS Backend = iota + 2
const CLNGRPC Backend = iota + 3
const FAKEWALLET Backend = iota + 4

type LightningBackend interface {
	PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error)
	CheckPayed(quote string) (cashu.ACTION_STATE, string, error)
	QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error)
	RequestInvoice(amount int64) (InvoiceResponse, error)
	WalletBalance() (uint64, error)
	LightningType() Backend
	GetNetwork() *chaincfg.Params
	// TODO CHECK that the inner pointer change work on network
	ChangeNetwork(network chaincfg.Params)
	ActiveMPP() bool
}
type PaymentResponse struct {
	Preimage       string
	PaymentError   error
	PaymentRequest string
	Rhash          string
	PaidFeeSat     int64
}
type QueryRoutesResponse struct {
	FeeReserve uint64 `json:"fee_reserve"`
}

type InvoiceResponse struct {
	PaymentRequest string
	Rhash          string
}

type FeesResponse struct {
	FeeReserve uint64 `json:"fee_reserve"`
}
