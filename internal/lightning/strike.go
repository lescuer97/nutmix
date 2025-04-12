package lightning

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type Strike struct {
	Network  chaincfg.Params
	endpoint string
	key      string
}

type strikeAccountBalanceResponse struct {
	Currency strikeCurrency `json:"currency"`
	Current  string         `json:"current"`
}
type strikeInvoiceRequest struct {
	CorrelationId string       `json:"correlationId"`
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

// C0B9B39D8A69F62647352D1E048B07E0A788E0FDE77623BFBD31BC97AB743703

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
type strikeErrorStatus struct {
	TraceId *string `json:"TraceId,omitempty"`
	Data    struct {
		Code    string `json:"string"`
		Message string `json:"message"`
		Status  uint   `json:"status"`
	} `json:"data"`
}

func (l *Strike) Setup(key string, endpoint string) error {
	if key == "" {
		return fmt.Errorf("Strike key not available")
	}

	if endpoint == "" {
		return fmt.Errorf("STRIKE endpoint not available")
	}
	l.key = key
	l.endpoint = endpoint

	return nil
}

func (l *Strike) StrikeRequest(method string, endpoint string, reqBody any, responseType any) error {
	client := &http.Client{}
	marshalledBody := bytes.NewBuffer(nil)
	if reqBody != nil {
		jsonBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("json.Marshal: %w", err)
		}
		marshalledBody = bytes.NewBuffer(jsonBytes)
	}
	fullUrl := l.endpoint + endpoint
	fullUrl = strings.TrimSpace(fullUrl)

	req, err := http.NewRequest(method, fullUrl, marshalledBody)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", l.key))
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(req): %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("io.ReadAll(resp.Body): %w", err)
	}

	switch resp.StatusCode {
	case 200, 201:
		err = json.Unmarshal(body, &responseType)
		if err != nil {
			return fmt.Errorf("json.Unmarshal(body, &responseType): %w", err)
		}
		return nil

	default:
		errorBody := strikeErrorStatus{}
		err = json.Unmarshal(body, &errorBody)
		if err != nil {
			return fmt.Errorf("json.Unmarshal(errorBody): %w", err)
		}

		switch errorBody.Data.Status {
		case 400:
			return fmt.Errorf("Bad request %+v, %+v", errorBody, reqBody)
		case 401:
			return fmt.Errorf("Unauthorized %+v", errorBody)
		default:
			return fmt.Errorf("Unknown error %+v", errorBody)

		}

	}

	return nil
}

func (l Strike) convertToSatAmount(amount strikeAmount) (uint64, error) {
	fee, err := strconv.ParseFloat(amount.Amount, 64)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseUint(fee_str, 10, 64): %w", err)
	}

	if amount.Currency == BTC {
		fee = fee * 1e8
	} else if amount.Currency == EUR {
		fee = fee * 100
	}

	return uint64(fee), nil
}

func (l Strike) PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var strikePayment strikePaymentStatus
	var invoiceRes PaymentResponse

	err := l.StrikeRequest("PATCH", fmt.Sprintf("/v1/payment-quotes/%s/execute", melt_quote.CheckingId), nil, &strikePayment)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
	}

	fee, err := l.convertToSatAmount(strikePayment.LightningNetworkFee)
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
	invoiceRes.PaymentRequest = melt_quote.Request
	invoiceRes.Rhash = hex.EncodeToString(payHash[:])

	return invoiceRes, nil
}

func (l Strike) CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, uint64, error) {
	var paymentStatus strikePaymentStatus

	err := l.StrikeRequest("GET", "/v1/payments/"+checkingId, nil, &paymentStatus)
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
func (l Strike) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	var paymentStatus strikeInvoiceResponse
	log.Printf("receive check %+v", quote)

	err := l.StrikeRequest("GET", fmt.Sprintf("/v1/invoices/%s", quote.CheckingId), nil, &paymentStatus)
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

func (l Strike) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (uint64, string, error) {
	var queryResponse strikePaymentQuoteResponse

	strikeAmount, err := CashuAmountToStrikeAmount(amount)
	if err != nil {
		return 0, "", fmt.Errorf(`CashuAmountToStrikeAmount(): %w`, err)
	}
	strikeQuery := strikePaymentRequest{
		LnInvoice:      invoice,
		SourceCurrency: strikeAmount.Currency,
	}

	invoiceString := "/v1/payment-quotes/lightning"

	err = l.StrikeRequest("POST", invoiceString, strikeQuery, &queryResponse)
	if err != nil {
		return 0, "", fmt.Errorf(`l.StrikeRequest("GET", invoiceString, nil, &queryResponse): %w`, err)
	}

	fee, err := l.convertToSatAmount(queryResponse.TotalFee)
	if err != nil {
		return 0, "", fmt.Errorf(`l.fee(queryResponse.TotalFee) %w`, err)
	}

	fee = GetFeeReserve(amount.Amount, fee)
	return fee, queryResponse.PaymentQuoteId.String(), nil
}

func (l Strike) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
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
		CorrelationId: quote.Quote,
		Description:   "",
		Amount:        strikeAmt,
	}

	// get invoice request
	var strikeInvoiceResponse strikeInvoiceResponse
	err = l.StrikeRequest("POST", "/v1/invoices", reqInvoice, &strikeInvoiceResponse)
	if err != nil {
		return response, fmt.Errorf(`l.StrikeRequest("POST", "/v1/invoices", r: %w`, err)
	}
	log.Printf("\n strike invoice: %+v", strikeInvoiceResponse)

	// get invoice quote
	var strikeInvoiceQuoteResponse strikeInvoiceQuoteResponse
	err = l.StrikeRequest("POST", fmt.Sprintf("/v1/invoices/%s/quote", strikeInvoiceResponse.InvoiceId.String()), nil, &strikeInvoiceQuoteResponse)
	if err != nil {
		return response, fmt.Errorf(`l.StrikeRequest("GET",fmt.Sprintf("/v1/invoices/", strikeInvoiceResponse.InvoiceId.String()), nil, &strikeInvoiceQuoteResponse): %w`, err)
	}

	response.PaymentRequest = strikeInvoiceQuoteResponse.LnInvoice
	response.Rhash = strikeInvoiceQuoteResponse.QuoteId
	response.CheckingId = strikeInvoiceResponse.InvoiceId.String()

	return response, nil
}

func (l Strike) WalletBalance() (uint64, error) {
	var balance []strikeAccountBalanceResponse
	err := l.StrikeRequest("GET", "/v1/balances", nil, &balance)
	if err != nil {
		return 0, fmt.Errorf(`l.StrikeRequest("GET", "/v1/balances": %w`, err)
	}

	balanceTotal := uint64(0)

	for _, bal := range balance {
		if bal.Currency == BTC {
			currentBalance, err := l.convertToSatAmount(strikeAmount{Amount: bal.Current, Currency: BTC})
			if err != nil {
				return 0, fmt.Errorf(`l.convertToSatAmount(strikeAmount{Amount: bal.Current, Currency: BTC}). %w`, err)
			}
			balanceTotal += currentBalance

		}

	}

	return balanceTotal * 1000, nil
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
