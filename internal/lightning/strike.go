package lightning

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/zpay32"
)

type Strike struct {
	Network  chaincfg.Params
	Endpoint string
	Key      string
}

// type LNBitsDetailErrorData struct {
// 	Detail string
// 	Status string
// }
// type lnbitsInvoiceRequest struct {
// 	Amount int64  `json:"amount"`
// 	Unit   string `json:"unit,omitempty"`
// 	Memo   string `json:"memo"`
// 	Out    bool   `json:"out"`
// 	Expiry int64  `json:"expiry"`
// 	Bolt11 string `json:"bolt11"`
// }
//
// type LNBitsPaymentStatusDetail struct {
// 	Memo    string
// 	Fee     int64
// 	Pending bool
// }
// type LNBitsPaymentStatus struct {
// 	Paid     bool   `json:"paid"`
// 	Pending  bool   `json:"pending"`
// 	Preimage string `json:"preimage"`
// 	Details  LNBitsPaymentStatusDetail
// }
// type lnbitsFeeResponse struct {
// 	FeeReserve uint64 `json:"fee_reserve"`
// }

// var ErrLnbitsFailedPayment = errors.New("failed payment")

func (l *Strike) LnbitsRequest(method string, endpoint string, reqBody any, responseType any) error {
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

	case detailBody.Detail == "Payment does not exist.":
	case len(detailBody.Detail) > 0:
		return fmt.Errorf("LNBITS Unknown error %+v. Request Body %+v", detailBody, reqBody)
	}

	err = json.Unmarshal(body, &responseType)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	return nil

}

func (l Strike) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error) {
	var invoiceRes PaymentResponse

	var lnbitsInvoice struct {
		PaymentHash    string `json:"payment_hash"`
		PaymentRequest string `json:"payment_request"`
	}
	reqInvoice := lnbitsInvoiceRequest{
		Out:    true,
		Bolt11: invoice,
		Amount: int64(amount_sat),
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
	invoiceRes.PaidFeeSat = int64(math.Abs(float64(paymentStatus.Details.Fee)))

	return invoiceRes, nil
}

func (l Strike) CheckPayed(quote string) (PaymentStatus, string, uint64, error) {
	var paymentStatus LNBitsPaymentStatus

	err := l.LnbitsRequest("GET", "/api/v1/payments/"+quote, nil, &paymentStatus)
	if err != nil {
		return FAILED, "", uint64(paymentStatus.Details.Fee), fmt.Errorf("json.Marshal: %w", err)
	}

	switch {
	case paymentStatus.Paid:
		return SETTLED, paymentStatus.Preimage, uint64(paymentStatus.Details.Fee), nil
	default:
		return PENDING, paymentStatus.Preimage, uint64(paymentStatus.Details.Fee), nil

	}
}
func (l Strike) CheckReceived(quote string) (PaymentStatus, string, error) {
	var paymentStatus LNBitsPaymentStatus

	err := l.LnbitsRequest("GET", "/api/v1/payments/"+quote, nil, &paymentStatus)
	if err != nil {
		return FAILED, "", fmt.Errorf("json.Marshal: %w", err)
	}

	switch {
	case paymentStatus.Paid:
		return SETTLED, paymentStatus.Preimage, nil
	default:
		return PENDING, paymentStatus.Preimage, nil

	}
}

func (l Strike) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error) {
	var queryResponse lnbitsFeeResponse
	invoiceString := "/api/v1/payments/fee-reserve" + "?" + `invoice=` + invoice

	err := l.LnbitsRequest("GET", invoiceString, nil, &queryResponse)
	queryResponse.FeeReserve = queryResponse.FeeReserve / 1000

	if err != nil {
		return 0, fmt.Errorf("json.Marshal: %w", err)
	}

	fee := GetFeeReserve(amount_sat, queryResponse.FeeReserve)
	return fee, nil
}

func (l Strike) RequestInvoice(amount int64) (InvoiceResponse, error) {
	reqInvoice := lnbitsInvoiceRequest{
		Amount: amount,
		Unit:   cashu.Sat.String(),
		Memo:   "",
		Out:    false,
		Expiry: 900,
	}
	var response InvoiceResponse
	var lnbitsInvoice struct {
		PaymentHash    string `json:"payment_hash"`
		PaymentRequest string `json:"payment_request"`
	}
	err := l.LnbitsRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
	if err != nil {
		return response, fmt.Errorf("json.Marshal: %w", err)
	}

	response.PaymentRequest = lnbitsInvoice.PaymentRequest
	response.Rhash = lnbitsInvoice.PaymentHash

	return response, nil

}

func (l Strike) WalletBalance() (uint64, error) {
	var channelBalance struct {
		Id      string `json:"id"`
		Name    string `json:"name"`
		Balance int    `json:"balance"`
	}
	err := l.LnbitsRequest("GET", "/api/v1/wallet", nil, &channelBalance)
	if err != nil {
		return 0, fmt.Errorf("l.LnbitsInvoiceRequest: %w", err)
	}

	return uint64(channelBalance.Balance), nil
}

func (f Strike) LightningType() Backend {
	return LNBITS
}

func (f Strike) GetNetwork() *chaincfg.Params {
	return &f.Network
}
func (f Strike) ActiveMPP() bool {
	return false
}
func (f Strike) VerifyUnitSupport(unit cashu.Unit) bool {
	if unit == cashu.Sat {
		return true
	} else {
		return false
	}
}
