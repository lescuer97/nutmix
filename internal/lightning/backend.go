package lightning

import (
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

var (
	ErrAlreadyPaid = errors.New("Invoice already paid")
)

type Backend uint

const LNDGRPC Backend = iota + 1
const LNBITS Backend = iota + 2
const CLNGRPC Backend = iota + 3
const FAKEWALLET Backend = iota + 4
const STRIKE Backend = iota + 5

type LightningBackend interface {
	PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error)
	CheckPayed(quote string) (PaymentStatus, string, uint64, error)
	CheckReceived(quote string) (PaymentStatus, string, error)
	QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (uint64, error)
	RequestInvoice(amount cashu.Amount) (InvoiceResponse, error)
	WalletBalance() (uint64, error)
	LightningType() Backend
	GetNetwork() *chaincfg.Params
	ActiveMPP() bool
	VerifyUnitSupport(unit cashu.Unit) bool
}

type PaymentStatus uint

const SETTLED PaymentStatus = iota + 1
const FAILED PaymentStatus = iota + 2
const PENDING PaymentStatus = iota + 3
const UNKNOWN PaymentStatus = iota + 999

type PaymentResponse struct {
	Preimage       string
	PaymentRequest string
	PaymentState   PaymentStatus
	Rhash          string
	PaidFeeSat     int64
}

type InvoiceResponse struct {
	PaymentRequest string
	Rhash          string
}
