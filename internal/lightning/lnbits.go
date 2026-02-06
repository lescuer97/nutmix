package lightning

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type LnbitsWallet struct {
	Endpoint string
	Key      string
	Network  chaincfg.Params
}

type LNBitsDetailErrorData struct {
	Detail string
	Status string
}
type lnbitsInvoiceRequest struct {
	Unit       string `json:"unit,omitempty"`
	CheckingId string `json:"checking_id,omitempty"`
	Memo       string `json:"memo"`
	Bolt11     string `json:"bolt11"`
	Amount     uint64 `json:"amount"`
	Expiry     int64  `json:"expiry"`
	Out        bool   `json:"out"`
}

type LNBitsPaymentStatusDetail struct {
	Memo    string
	Status  string
	Fee     int64
	Pending bool
}
type LNBitsPaymentStatus struct {
	Preimage string `json:"preimage"`
	Details  LNBitsPaymentStatusDetail
	Paid     bool `json:"paid"`
	Pending  bool `json:"pending"`
}
type lnbitsFeeResponse struct {
	FeeReserve uint64 `json:"fee_reserve"`
}

var ErrLnbitsFailedPayment = errors.New("failed payment")
var ErrLnBitsNoRouteFound = errors.New("no route found")

func (l *LnbitsWallet) LnbitsRequest(method string, endpoint string, reqBody any, responseType any) error {
	client := &http.Client{}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	b := bytes.NewBuffer(jsonBytes)

	req, err := http.NewRequest(method, l.Endpoint+endpoint, b)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("X-Api-Key", l.Key)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(req): %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("failed to close response body", slog.Any("error", err))
		}
	}()

	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %w", err)
	}

	detailBody := LNBitsDetailErrorData{}
	err = json.Unmarshal(body, &detailBody)
	if err != nil {
		return fmt.Errorf("json.Unmarshal(detailBody): %w", err)
	}

	switch {
	case detailBody.Status == "failed":
		return fmt.Errorf("LNBITS payment failed %+v. Request Body %+v, %w", detailBody, reqBody, ErrLnbitsFailedPayment)

	case detailBody.Detail == "Payment failed: no_route":
		return fmt.Errorf("no route found %+v. Request Body %+v, %w", detailBody, reqBody, ErrLnBitsNoRouteFound)

	case detailBody.Detail == "Payment does not exist.":
		val, ok := responseType.(LNBitsPaymentStatus)
		if ok {
			val.Paid = false
			val.Pending = false
			val.Details.Pending = false
			responseType = val
			return nil
		}
	case len(detailBody.Detail) > 0:
		return fmt.Errorf("LNBITS Unknown error %+v. Request Body %+v. body:%s. %w ", detailBody, reqBody, body, ErrLnbitsFailedPayment)
	}

	err = json.Unmarshal(body, &responseType)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	return nil
}

func (l LnbitsWallet) PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve cashu.Amount, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var invoiceRes PaymentResponse

	var lnbitsInvoice struct {
		PaymentHash    string `json:"payment_hash"`
		PaymentRequest string `json:"payment_request"`
	}

	reqInvoice := lnbitsInvoiceRequest{
		Out:    true,
		Bolt11: melt_quote.Request,
		Amount: amount.Amount,
	}

	err := l.LnbitsRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
	}

	var paymentStatus LNBitsPaymentStatus

	// check invoice payment to get the preimage and fee
	err = l.LnbitsRequest("GET", "/api/v1/payments/"+lnbitsInvoice.PaymentHash, nil, &paymentStatus)
	if err != nil {
		if errors.Is(err, ErrLnbitsFailedPayment) {
			invoiceRes.PaymentState = FAILED
		}
		return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("GET", "/api/v1/payments/"+lnbitsInvoice.PaymentHash, nil, &paymentStatus): %w`, err)
	}

	invoiceRes.PaymentRequest = lnbitsInvoice.PaymentRequest
	invoiceRes.PaymentState = SETTLED
	invoiceRes.Rhash = lnbitsInvoice.PaymentHash
	invoiceRes.Preimage = paymentStatus.Preimage
	// LNBits returns fee as int64, convert to Amount
	invoiceRes.PaidFee = cashu.NewAmount(amount.Unit, uint64(math.Abs(float64(paymentStatus.Details.Fee))))
	invoiceRes.CheckingId = melt_quote.CheckingId

	return invoiceRes, nil
}

func (l LnbitsWallet) CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, cashu.Amount, error) {
	var paymentStatus LNBitsPaymentStatus
	zeroFee := cashu.NewAmount(cashu.Sat, 0)

	hash := invoice.PaymentHash[:]
	err := l.LnbitsRequest("GET", "/api/v1/payments/"+hex.EncodeToString(hash), nil, &paymentStatus)
	if err != nil {
		return FAILED, "", zeroFee, fmt.Errorf("json.Marshal: %w", err)
	}

	fee := cashu.NewAmount(cashu.Sat, uint64(paymentStatus.Details.Fee))

	switch {
	case paymentStatus.Paid:
		return SETTLED, paymentStatus.Preimage, fee, nil
	case paymentStatus.Details.Status == "pending":
		return PENDING, paymentStatus.Preimage, fee, nil
	case paymentStatus.Details.Pending:
		return PENDING, paymentStatus.Preimage, fee, nil
	case !paymentStatus.Paid && paymentStatus.Details.Status == "failed":
		return FAILED, paymentStatus.Preimage, fee, nil
	case !paymentStatus.Paid && !paymentStatus.Details.Pending:
		return FAILED, paymentStatus.Preimage, fee, nil
	default:
		return FAILED, paymentStatus.Preimage, fee, nil
	}
}

func (l LnbitsWallet) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	var paymentStatus LNBitsPaymentStatus

	hash := invoice.PaymentHash[:]

	err := l.LnbitsRequest("GET", "/api/v1/payments/"+hex.EncodeToString(hash), nil, &paymentStatus)
	if err != nil {
		return FAILED, "", fmt.Errorf("json.Marshal: %w", err)
	}

	switch {
	case paymentStatus.Paid:
		return SETTLED, paymentStatus.Preimage, nil
	case paymentStatus.Details.Pending:
		return PENDING, paymentStatus.Preimage, nil
	case !paymentStatus.Paid && !paymentStatus.Details.Pending:
		return FAILED, paymentStatus.Preimage, nil
	default:
		return FAILED, paymentStatus.Preimage, nil
	}
}

func (l LnbitsWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error) {
	var queryResponse lnbitsFeeResponse
	invoiceString := "/api/v1/payments/fee-reserve" + "?" + `invoice=` + invoice

	err := l.LnbitsRequest("GET", invoiceString, nil, &queryResponse)

	feesResponse := FeesResponse{}
	if err != nil {
		return feesResponse, fmt.Errorf("json.Marshal: %w", err)
	}

	// LNBits returns fee in msats, convert to Amount
	feeMsat := cashu.NewAmount(cashu.Msat, queryResponse.FeeReserve)
	convertErr := feeMsat.To(amount.Unit)
	if convertErr != nil {
		return feesResponse, fmt.Errorf("feeMsat.To(amount.Unit): %w", convertErr)
	}

	fee := GetFeeReserve(amount.Amount, feeMsat.Amount)
	hash := zpayInvoice.PaymentHash[:]

	feesResponse.Fees = cashu.NewAmount(amount.Unit, fee)
	feesResponse.AmountToSend = amount
	feesResponse.CheckingId = hex.EncodeToString(hash)

	return feesResponse, nil
}

func (l LnbitsWallet) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
	// Convert amount to Sat for LNBits
	amountSat := cashu.NewAmount(amount.Unit, amount.Amount)
	err := amountSat.To(cashu.Sat)
	if err != nil {
		return InvoiceResponse{}, fmt.Errorf(`amount.To(cashu.Sat) %w`, err)
	}

	reqInvoice := lnbitsInvoiceRequest{
		Amount: amountSat.Amount,
		Unit:   cashu.Sat.String(),
		Memo:   "",
		Out:    false,
		Expiry: 900,
	}

	if quote.Description != nil {
		reqInvoice.Memo = *quote.Description
	}

	var response InvoiceResponse

	supported := l.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit). %w", cashu.ErrUnitNotSupported)
	}

	var lnbitsInvoice struct {
		PaymentHash    string `json:"payment_hash"`
		PaymentRequest string `json:"payment_request"`
		Bolt11         string `json:"bolt11"`
	}
	err = l.LnbitsRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
	if err != nil {
		return response, fmt.Errorf("json.Marshal: %w", err)
	}

	if lnbitsInvoice.Bolt11 != "" {

		response.PaymentRequest = lnbitsInvoice.Bolt11
	} else {
		response.PaymentRequest = lnbitsInvoice.PaymentRequest
	}

	response.Rhash = lnbitsInvoice.PaymentHash
	response.CheckingId = lnbitsInvoice.PaymentHash

	return response, nil

}

func (l LnbitsWallet) WalletBalance() (cashu.Amount, error) {
	var channelBalance struct {
		Id      string `json:"id"`
		Name    string `json:"name"`
		Balance int    `json:"balance"`
	}
	err := l.LnbitsRequest("GET", "/api/v1/wallet", nil, &channelBalance)
	if err != nil {
		return cashu.Amount{}, fmt.Errorf("l.LnbitsInvoiceRequest: %w", err)
	}

	// LNBits returns balance in sats
	return cashu.NewAmount(cashu.Sat, uint64(channelBalance.Balance)), nil
}

func (f LnbitsWallet) LightningType() Backend {
	return LNBITS
}

func (f LnbitsWallet) GetNetwork() *chaincfg.Params {
	return &f.Network
}
func (f LnbitsWallet) ActiveMPP() bool {
	return false
}
func (f LnbitsWallet) VerifyUnitSupport(unit cashu.Unit) bool {
	switch unit {
	case cashu.Sat:
		return true
	default:
		return false
	}
}

func (f LnbitsWallet) DescriptionSupport() bool {
	return true
}
