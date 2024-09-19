package lightning

type Backend uint

const LNDGRPC Backend = iota + 1
const LNBITS Backend = iota + 2
const CLNGRPC Backend = iota + 3
const FAKEWALLET Backend = iota + 4

type LightningBackend interface {
	PayInvoice() (PaymentResponse, error)
	CheckPayed()
	QueryFees()
	RequestInvoice(amount int64)
	WalletBalance() (uint64, error)
	LightningType() Backend
}
type PaymentResponse struct {
	Preimage       string
	PaymentError   error
	PaymentRequest string
	Rhash          string
	PaidFeeSat     int64
}

type InvoicePayment struct {
	PaymentRequest string
	Rhash          string
}

type FeesResponse struct {
	FeeReserve uint64 `json:"fee_reserve"`
}
