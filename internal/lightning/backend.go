package lightning

import (
	"context"
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

var (
	ErrAlreadyPaid        = errors.New("invoice already paid")
	ErrLNBackendEndOfLife = errors.New("lightning backend is end of life")
)

type Backend uint

const LNDGRPC Backend = iota + 1

// Deprecated: LNBITS backend will be removed in v0.8.0.
const LNBITS Backend = iota + 2
const CLNGRPC Backend = iota + 3
const FAKEWALLET Backend = iota + 4

// Deprecated: Strike backend will be removed in v0.7.0.
const STRIKE Backend = iota + 5

// Check what is the current status of a node.
type NodeStatus string

const ONLINE_STATUS NodeStatus = "ONLINE"
const OFFLINE_STATUS NodeStatus = "OFFLINE"
const UNKNOWN_STATUS NodeStatus = "UNKNOWN"
const STOPPED_STATUS NodeStatus = "STOPPED"

type LightningBackend interface {
	PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve cashu.Amount, mpp bool, amount cashu.Amount) (PaymentResponse, error)
	CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, cashu.Amount, error)
	CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error)
	RequestInvoice(amount cashu.Amount, description *string) (InvoiceResponse, error)
	// returns the amount in sats and the checking_id
	QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error)
	// returns milisats balance
	WalletBalance() (cashu.Amount, error)
	LightningType() Backend
	GetNetwork() *chaincfg.Params
	Status(ctx context.Context) (NodeStatus, error)
	ActiveMPP() bool
	VerifyUnitSupport(unit cashu.Unit) bool
	DescriptionSupport() bool
}

func IsBackendEndOfLife(backend LightningBackend) bool {
	if backend == nil {
		return false
	}

	switch backend.LightningType() {
	case STRIKE:
		return true
	default:
		return false
	}
}

type PaymentStatus uint

const SETTLED PaymentStatus = iota + 1
const FAILED PaymentStatus = iota + 2
const PENDING PaymentStatus = iota + 3
const UNKNOWN PaymentStatus = iota + 999

type PaymentResponse struct {
	Preimage       string
	PaymentRequest string
	Rhash          string
	CheckingId     string
	PaymentState   PaymentStatus
	PaidFee        cashu.Amount
}
type FeesResponse struct {
	CheckingId   string
	Fees         cashu.Amount
	AmountToSend cashu.Amount
}

type InvoiceResponse struct {
	PaymentRequest string
	CheckingId     string
	Rhash          string
}
