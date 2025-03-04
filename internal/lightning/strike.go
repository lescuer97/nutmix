package lightning

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type Strike struct {
	Network  chaincfg.Params
	Endpoint string
	Key      string
}

type strikeAccountBalanceResponse struct {
	Currency strikeCurrency `json:"currency"`
	Current  string         `json:"current"`
}
type strikeInvoiceRequest struct {
	CorrelationId uuid.UUID    `json:"correlationId"`
	Description   string       `json:"description"`
	Amount        strikeAmount `json:"amount"`
}

type strikeInvoiceState string

const UNPAID strikeInvoiceState = "UNPAID"
const PAID strikeInvoiceState = "PAID"
const PENDING_STRIKE strikeInvoiceState = "PENDING"
const CANCELLED strikeInvoiceState = "CANCELLED"

type strikePaymentState string

const PENDING_STRIKE_PAYMENT strikePaymentState = "PENDING"
const COMPLETED strikePaymentState = "COMPLETED"
const FAILED_STRIKE_PAYMENT strikePaymentState = "FAILED"

type strikeCurrency string

const BTC strikeCurrency = "BTC"
const USD strikeCurrency = "USD"
const EUR strikeCurrency = "EUR"

type strikeAmount struct {
	Amount   string         `json:"amount"`
	Currency strikeCurrency `json:"currency"`
}

func strikeInvoiceStateToCashuState(state strikeInvoiceState) (PaymentStatus, error) {
	switch state {
	case UNPAID:
		return UNKNOWN, nil
	case PAID:
		return SETTLED, nil
	case PENDING_STRIKE:
		return PENDING, nil
	case CANCELLED:
		return FAILED, nil
	default:
		return PENDING, fmt.Errorf("Could not get payement status from strike state")
	}
}

func strikePaymentStateToCashuState(state strikePaymentState) (PaymentStatus, error) {
	switch state {
	case PENDING_STRIKE_PAYMENT:
		return PENDING, nil
	case COMPLETED:
		return SETTLED, nil
	case FAILED_STRIKE_PAYMENT:
		return FAILED, nil
	default:
		return PENDING, fmt.Errorf("Could not get payement status from strike state")
	}
}

func CashuAmountToStrikeAmount(amount cashu.Amount) (strikeAmount, error) {
	var strikeAmt strikeAmount
	floatStr, err := amount.ToFloatString()
	if err != nil {
		return strikeAmt, fmt.Errorf("amount.ToFloatString(): %w", err)
	}
	switch amount.Unit {
	case cashu.Sat:
		return strikeAmount{
			Amount:   floatStr,
			Currency: BTC,
		}, nil
	case cashu.EUR:
		return strikeAmount{
			Amount:   floatStr,
			Currency: EUR,
		}, nil

	}
	return strikeAmt, cashu.ErrCouldNotConvertUnit

}

type strikeInvoiceResponse struct {
	InvoiceId   uuid.UUID          `json:"invoiceId"`
	Description string             `json:"description"`
	Amount      strikeAmount       `json:"amount"`
	State       strikeInvoiceState `json:"state"`
}
type strikeInvoiceQuoteResponse struct {
	QuoteId         string       `json:"quoteId"`
	Description     string       `json:"description"`
	LnInvoice       string       `json:"lnInvoice"`
	Expiration      string       `json:"expiration"`
	ExpirationInSec int64        `json:"expirationInSec"`
	TargetAmount    strikeAmount `json:"targetAmount"`
}

type strikePaymentRequest struct {
	LnInvoice      string         `json:"lnInvoice"`
	SourceCurrency strikeCurrency `json:"sourceCurrency"`
	Amount         strikeAmount   `json:"amount"`
}

type strikePaymentQuoteResponse struct {
	PaymentQuoteId      uuid.UUID    `json:"paymentQuoteId"`
	LightningNetworkFee strikeAmount `json:"lightningNetworkFee"`
	Amount              strikeAmount `json:"amount"`
	TotalFee            strikeAmount `json:"totalFee"`
	TotalAmount         strikeAmount `json:"totalAmount"`
}

type strikePaymentStatus struct {
	PaymentId           string             `json:"paymentId"`
	State               strikePaymentState `json:"state"`
	Completed           uint64             `json:"completed"`
	Amount              strikeAmount       `json:"amount"`
	TotalFee            strikeAmount       `json:"totalFee"`
	LightningNetworkFee strikeAmount       `json:"lightningNetworkFee"`
	Lightning           struct {
		NetworkFee strikeAmount `json:"networkFee"`
	} `json:"lightning"`
}

func (l *Strike) StrikeRequest(method string, endpoint string, reqBody any, responseType any) error {
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
	// req.H
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", l.Key))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(req): %w", err)
	}

	body, err := io.ReadAll(resp.Body)

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
		return fmt.Errorf("Strike payment failed %+v. Request Body %+v, %w", detailBody, reqBody, ErrLnbitsFailedPayment)

	case detailBody.Detail == "Payment does not exist.":
	case len(detailBody.Detail) > 0:
		return fmt.Errorf("strike Unknown error %+v. Request Body %+v", detailBody, reqBody)
	}

	err = json.Unmarshal(body, &responseType)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	return nil

}

func (l Strike) fee(amount strikeAmount) (uint64, error) {
	fee, err := strconv.ParseUint(amount.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseUint(fee_str, 10, 64): %w", err)
	}

	if amount.Currency == BTC {
		fee = fee * 1e8
	} else if amount.Currency == EUR {
		fee = fee * 100
	}

	return fee, nil
}

func (l Strike) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var strikePayment strikePaymentStatus
	var invoiceRes PaymentResponse

	err := l.StrikeRequest("PATCH", fmt.Sprintf("/v1/payment-quotes/%s/execute"), nil, &strikePayment)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
	}

	fee, err := l.fee(strikePayment.LightningNetworkFee)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.fee(queryResponse.TotalFee) %w`, err)
	}

	state, err := strikePaymentStateToCashuState(strikePayment.State)
	if err != nil {
		return invoiceRes, fmt.Errorf(`strikeStateToCashuState(strikePayment.State) %w`, err)
	}
	payHash := *zpayInvoice.PaymentHash
	invoiceRes.PaidFeeSat = int64(fee)
	invoiceRes.PaymentState = state
	invoiceRes.PaymentRequest = invoice
	invoiceRes.Rhash = hex.EncodeToString(payHash[:])

	return invoiceRes, nil
}

func (l Strike) CheckPayed(quote string) (PaymentStatus, string, uint64, error) {
	var paymentStatus strikePaymentStatus

	err := l.StrikeRequest("GET", "/v1/payments/"+quote, nil, &paymentStatus)
	if err != nil {
		return FAILED, "", uint64(0), fmt.Errorf(`l.StrikeRequest("GET", "/api/v1/payments/"+quote: %w`, err)
	}

	lnFee, err := strconv.ParseUint(paymentStatus.LightningNetworkFee.Amount, 10, 64)
	if err != nil {
		return FAILED, "", uint64(0), fmt.Errorf(`strconv.ParseUint(paymentStatus.LightningNetworkFee, 10, 64): %w`, err)
	}

	state, err := strikePaymentStateToCashuState(paymentStatus.State)
	if err != nil {
		return PENDING, "", lnFee, fmt.Errorf("strikePaymentStateToCashuState(strikePayment.State): %w", err)
	}
	return state, "", lnFee, nil
}
func (l Strike) CheckReceived(quote string) (PaymentStatus, string, error) {
	var paymentStatus strikeInvoiceResponse

	err := l.StrikeRequest("GET", fmt.Sprintf("/v1/invoices/%s", quote), nil, &paymentStatus)
	if err != nil {
		return FAILED, "", fmt.Errorf(`l.StrikeRequest("GET", fmt.Sprintf("/v1/invoices/", quote), nil, &paymentStatus) %w`, err)
	}

	switch paymentStatus.State {
	case UNPAID:
		return FAILED, "", nil
	case PAID:
		return SETTLED, "", nil
	case PENDING_STRIKE:
		return PENDING, "", nil
	case CANCELLED:
		return FAILED, "", nil
	default:
		return PENDING, "", nil
	}
}

func (l Strike) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (uint64, error) {
	var queryResponse strikePaymentQuoteResponse
	invoiceString := "/api/v1/payments/fee-reserve" + "?" + `invoice=` + invoice

	err := l.StrikeRequest("GET", invoiceString, nil, &queryResponse)
	if err != nil {
		return 0, fmt.Errorf(`l.StrikeRequest("GET", invoiceString, nil, &queryResponse): %w`, err)
	}

	fee, err := l.fee(queryResponse.TotalFee)
	if err != nil {
		return 0, fmt.Errorf(`l.fee(queryResponse.TotalFee) %w`, err)
	}

	fee = GetFeeReserve(amount.Amount, fee)
	return fee, nil
}

func (l Strike) RequestInvoice(amount cashu.Amount) (InvoiceResponse, error) {
	uuid := uuid.New()

	var response InvoiceResponse
	supported := l.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit). %w", cashu.ErrUnitNotSupported)
	}

	strikeAmt, err := CashuAmountToStrikeAmount(amount)
	if err != nil {
		return response, fmt.Errorf("CashuAmountToStrikeAmount(amount): %w", err)
	}
	reqInvoice := strikeInvoiceRequest{
		CorrelationId: uuid,
		Description:   "",
		Amount:        strikeAmt,
	}

	// get invoice request
	var strikeInvoiceResponse strikeInvoiceResponse
	err = l.StrikeRequest("POST", "/v1/invoices", reqInvoice, &strikeInvoiceResponse)
	if err != nil {
		return response, fmt.Errorf(`l.StrikeRequest("POST", "/v1/invoices", r: %w`, err)
	}

	// get invoice quote
	var strikeInvoiceQuoteResponse strikeInvoiceQuoteResponse
	err = l.StrikeRequest("GET", fmt.Sprintf("/v1/invoices/%s", strikeInvoiceResponse.InvoiceId.String()), nil, &strikeInvoiceQuoteResponse)
	if err != nil {
		return response, fmt.Errorf(`l.StrikeRequest("GET",fmt.Sprintf("/v1/invoices/", strikeInvoiceResponse.InvoiceId.String()), nil, &strikeInvoiceQuoteResponse): %w`, err)
	}

	response.PaymentRequest = strikeInvoiceQuoteResponse.LnInvoice
	response.Rhash = strikeInvoiceQuoteResponse.QuoteId

	return response, nil
}

func (l Strike) WalletBalance() (uint64, error) {
	var balance strikeAccountBalanceResponse
	err := l.StrikeRequest("GET", "/v1/balances", nil, &balance)
	if err != nil {
		return 0, fmt.Errorf(`l.StrikeRequest("GET", "/v1/balances": %w`, err)
	}

	currentBalance, err := strconv.ParseUint(balance.Current, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(`strconv.ParseUint(balance.Current, 10, 64). %w`, err)
	}

	return currentBalance, nil
}

func (f Strike) LightningType() Backend {
	return STRIKE
}

func (f Strike) GetNetwork() *chaincfg.Params {
	return &f.Network
}

func (f Strike) ActiveMPP() bool {
	return false
}

func (f Strike) VerifyUnitSupport(unit cashu.Unit) bool {
	switch unit {
	case cashu.Sat:
		return true
	case cashu.EUR:
		return true
	default:
		return false
	}
}
