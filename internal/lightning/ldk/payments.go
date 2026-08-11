package ldk

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/zpay32"
)

const (
	outboundPaymentWaitTimeout  = 30 * time.Second
	outboundPaymentPollInterval = 200 * time.Millisecond
)

func (l *LDK) PayInvoice(meltQuote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve cashu.Amount, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var response PaymentResponse
	if zpayInvoice == nil {
		return response, fmt.Errorf("zpay invoice is nil")
	}
	if !l.VerifyUnitSupport(amount.Unit) {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit): %w", cashu.ErrUnitNotSupported)
	}

	amountMsat := amount
	if err := amountMsat.To(cashu.Msat); err != nil {
		return response, fmt.Errorf("amount.To(cashu.Msat) %w", err)
	}

	node, err := l.getNode()
	if err != nil {
		return response, err
	}
	if meltQuote.Request == "" {
		return response, fmt.Errorf("empty invoice request")
	}

	ldkInvoice, err := ldk_node.Bolt11InvoiceFromStr(meltQuote.Request)
	if err != nil {
		return response, fmt.Errorf("ldk_node.Bolt11InvoiceFromStr(meltQuote.Request) %w", err)
	}

	routeParams, err := l.buildRouteParameters(node, feeReserve, mpp)
	if err != nil {
		return response, err
	}

	bolt11 := node.Bolt11Payment()
	var paymentID ldk_node.PaymentId
	if zpayInvoice.MilliSat == nil || *zpayInvoice.MilliSat == 0 {
		if amountMsat.Amount == 0 {
			return response, fmt.Errorf("amount is not available for the invoice")
		}
		paymentID, err = bolt11.SendUsingAmount(ldkInvoice, amountMsat.Amount, routeParams)
		if err != nil {
			return response, fmt.Errorf("bolt11.SendUsingAmount(ldkInvoice, amountMsat.Amount, routeParams) %w", err)
		}
	} else {
		paymentID, err = bolt11.Send(ldkInvoice, routeParams)
		if err != nil {
			return response, fmt.Errorf("bolt11.Send(ldkInvoice, routeParams) %w", err)
		}
	}

	response.PaymentRequest = meltQuote.Request
	if paymentID != "" {
		response.CheckingId = paymentID
	} else {
		response.CheckingId = meltQuote.CheckingId
		if response.CheckingId == "" {
			response.CheckingId = ldkInvoice.PaymentHash()
		}
	}
	response.Rhash = ldkInvoice.PaymentHash()

	status, preimage, fee, err := l.waitForOutboundPayment(zpayInvoice, response.CheckingId, outboundPaymentWaitTimeout)
	if err != nil {
		return response, err
	}

	response.PaymentState = status
	response.Preimage = preimage
	response.PaidFee = fee
	return response, nil
}

func (l *LDK) waitForOutboundPayment(invoice *zpay32.Invoice, checkingID string, timeout time.Duration) (PaymentStatus, string, cashu.Amount, error) {
	status, preimage, fee, err := l.checkOutboundPaymentStatus(invoice, checkingID)
	if err != nil {
		return UNKNOWN, "", cashu.Amount{}, err
	}
	if status != PENDING || timeout <= 0 {
		return status, preimage, fee, nil
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(outboundPaymentPollInterval)
		status, preimage, fee, err = l.checkOutboundPaymentStatus(invoice, checkingID)
		if err != nil {
			return UNKNOWN, "", cashu.Amount{}, err
		}
		if status != PENDING {
			return status, preimage, fee, nil
		}
	}

	return status, preimage, fee, nil
}

func (l *LDK) buildRouteParameters(node *ldk_node.Node, feeReserve cashu.Amount, mpp bool) (*ldk_node.RouteParametersConfig, error) {
	if node == nil {
		return nil, fmt.Errorf("ldk node is nil")
	}

	var routeParams *ldk_node.RouteParametersConfig
	config := node.Config()
	if config.RouteParameters != nil {
		copyParams := *config.RouteParameters
		routeParams = &copyParams
	}
	if routeParams == nil {
		if feeReserve.Amount == 0 && !mpp {
			return nil, nil
		}
		routeParams = &ldk_node.RouteParametersConfig{
			MaxTotalRoutingFeeMsat:          nil,
			MaxTotalCltvExpiryDelta:         2016,
			MaxPathCount:                    10,
			MaxChannelSaturationPowerOfHalf: 0,
		}
	}
	if mpp && routeParams.MaxPathCount < 2 {
		routeParams.MaxPathCount = 5
	}
	if feeReserve.Amount > 0 {
		err := feeReserve.To(cashu.Msat)
		if err != nil {
			return nil, fmt.Errorf("could not convert feeReserve: %w", err)
		}
		routeParams.MaxTotalRoutingFeeMsat = &feeReserve.Amount
	}

	return routeParams, nil
}

func (l *LDK) CheckPayed(quote string, invoice *zpay32.Invoice, checkingID string) (PaymentStatus, string, cashu.Amount, error) {
	return l.waitForOutboundPayment(invoice, checkingID, outboundPaymentWaitTimeout)
}

func (l *LDK) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	return l.checkInboundPaymentStatus(invoice, quote.CheckingId)
}

func (l *LDK) checkOutboundPaymentStatus(invoice *zpay32.Invoice, checkingID string) (PaymentStatus, string, cashu.Amount, error) {
	details, err := l.lookupPaymentDetails(invoice, ldk_node.PaymentDirectionOutbound, checkingID)
	if err != nil {
		return UNKNOWN, "", cashu.Amount{}, err
	}

	status, preimage, fee, err := paymentStatusFromDetails(details)
	if err != nil {
		return UNKNOWN, "", cashu.Amount{}, err
	}

	return status, preimage, fee, nil
}

func (l *LDK) checkInboundPaymentStatus(invoice *zpay32.Invoice, checkingID string) (PaymentStatus, string, error) {
	details, err := l.lookupPaymentDetails(invoice, ldk_node.PaymentDirectionInbound, checkingID)
	if err != nil {
		return UNKNOWN, "", err
	}

	status, preimage, _, err := paymentStatusFromDetails(details)
	if err != nil {
		return UNKNOWN, "", err
	}

	return status, preimage, nil
}

func (l *LDK) lookupPaymentDetails(invoice *zpay32.Invoice, direction ldk_node.PaymentDirection, checkingID string) (*ldk_node.PaymentDetails, error) {
	if invoice == nil {
		return nil, fmt.Errorf("zpay invoice is nil")
	}

	node, err := l.getNode()
	if err != nil {
		return nil, err
	}

	hash := hex.EncodeToString(invoice.PaymentHash[:])
	var paymentByID *ldk_node.PaymentDetails
	if checkingID != "" {
		paymentByID = node.Payment(checkingID)
	}

	return findPaymentDetails(node.ListPayments(), paymentByID, direction, hash), nil
}

func findPaymentDetails(payments []ldk_node.PaymentDetails, paymentByID *ldk_node.PaymentDetails, direction ldk_node.PaymentDirection, hash string) *ldk_node.PaymentDetails {
	if paymentMatches(paymentByID, direction, hash) {
		return paymentByID
	}

	var best *ldk_node.PaymentDetails
	var bestTimestamp uint64
	for _, payment := range payments {
		if !paymentMatches(&payment, direction, hash) {
			continue
		}
		if best == nil || payment.LatestUpdateTimestamp > bestTimestamp {
			paymentCopy := payment
			best = &paymentCopy
			bestTimestamp = payment.LatestUpdateTimestamp
		}
	}

	return best
}

func paymentMatches(details *ldk_node.PaymentDetails, direction ldk_node.PaymentDirection, hash string) bool {
	if details == nil || details.Direction != direction {
		return false
	}
	if hash == "" {
		return true
	}

	paymentHash, ok := bolt11PaymentHash(details.Kind)
	if !ok {
		return false
	}
	return paymentHash == hash
}

func bolt11PaymentHash(kind ldk_node.PaymentKind) (string, bool) {
	switch payment := kind.(type) {
	case ldk_node.PaymentKindBolt11:
		return payment.Hash, true
	case *ldk_node.PaymentKindBolt11:
		if payment == nil {
			return "", false
		}
		return payment.Hash, true
	case ldk_node.PaymentKindBolt11Jit:
		return payment.Hash, true
	case *ldk_node.PaymentKindBolt11Jit:
		if payment == nil {
			return "", false
		}
		return payment.Hash, true
	default:
		return "", false
	}
}

func paymentStatusFromDetails(details *ldk_node.PaymentDetails) (PaymentStatus, string, cashu.Amount, error) {
	if details == nil {
		return PENDING, "", cashu.Amount{Amount: 0, Unit: cashu.Msat}, nil
	}

	status, err := paymentStatusFromLDK(details.Status)
	if err != nil {
		return UNKNOWN, "", cashu.Amount{}, err
	}
	preimage := ""
	switch payment := details.Kind.(type) {
	case ldk_node.PaymentKindBolt11:
		if payment.Preimage != nil {
			preimage = *payment.Preimage
		}
	case *ldk_node.PaymentKindBolt11:
		if payment == nil {
			break
		}
		if payment.Preimage != nil {
			preimage = *payment.Preimage
		}
	case ldk_node.PaymentKindBolt11Jit:
		if payment.Preimage != nil {
			preimage = *payment.Preimage
		}
	case *ldk_node.PaymentKindBolt11Jit:
		if payment == nil {
			break
		}
		if payment.Preimage != nil {
			preimage = *payment.Preimage
		}
	}

	feeAmount := uint64(0)
	if details.FeePaidMsat != nil {
		feeAmount = *details.FeePaidMsat
	}

	return status, preimage, cashu.Amount{Amount: feeAmount, Unit: cashu.Msat}, nil
}

func paymentStatusFromLDK(status ldk_node.PaymentStatus) (PaymentStatus, error) {
	switch status {
	case ldk_node.PaymentStatusSucceeded:
		return SETTLED, nil
	case ldk_node.PaymentStatusFailed:
		return FAILED, nil
	case ldk_node.PaymentStatusPending:
		return PENDING, nil
	default:
		return UNKNOWN, fmt.Errorf("unknown ldk payment status: %v", status)
	}
}

func (l *LDK) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
	ldkStorage := l.storageDir()
	log.Printf("\n ldkStorage inside invoice req: %+v\n ", ldkStorage)
	if !l.VerifyUnitSupport(amount.Unit) {
		return InvoiceResponse{}, fmt.Errorf("l.VerifyUnitSupport(amount.Unit): %w", cashu.ErrUnitNotSupported)
	}

	amountMsat := amount
	if err := amountMsat.To(cashu.Msat); err != nil {
		return InvoiceResponse{}, fmt.Errorf("amount.To(cashu.Msat) %w", err)
	}

	node, err := l.getNode()
	if err != nil {
		return InvoiceResponse{}, err
	}

	description := ""
	if quote.Description != nil {
		description = *quote.Description
	}
	invoiceDescription := ldk_node.Bolt11InvoiceDescriptionDirect{Description: description}
	const expirySeconds = 36000

	bolt11 := node.Bolt11Payment()
	ldkInvoice, err := bolt11.Receive(amountMsat.Amount, invoiceDescription, expirySeconds)
	if err != nil {
		return InvoiceResponse{}, fmt.Errorf("bolt11.Receive(amountMsat.Amount, invoiceDescription, expirySeconds) %w", err)
	}

	return InvoiceResponse{
		PaymentRequest: ldkInvoice.String(),
		CheckingId:     ldkInvoice.PaymentHash(),
		Rhash:          ldkInvoice.PaymentHash(),
	}, nil
}

func (l *LDK) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error) {
	if !l.VerifyUnitSupport(amount.Unit) {
		return FeesResponse{}, fmt.Errorf("l.VerifyUnitSupport(amount.Unit): %w", cashu.ErrUnitNotSupported)
	}

	amountMsat := amount
	if err := amountMsat.To(cashu.Msat); err != nil {
		return FeesResponse{}, fmt.Errorf("amount.To(cashu.Msat) %w", err)
	}
	amountSat := amount
	if err := amountSat.To(cashu.Sat); err != nil {
		return FeesResponse{}, fmt.Errorf("amount.To(cashu.Sat) %w", err)
	}

	fee := lightning.GetFeeReserve(amountSat.Amount, 0)
	feeAmount := cashu.Amount{Unit: cashu.Sat, Amount: fee}
	amountToSend := amountSat
	if amount.Unit == cashu.Msat {
		if err := feeAmount.To(cashu.Msat); err != nil {
			return FeesResponse{}, fmt.Errorf("feeAmount.To(cashu.Msat) %w", err)
		}
		amountToSend = amountMsat
	}

	return FeesResponse{
		Fees:         feeAmount,
		AmountToSend: amountToSend,
		CheckingId:   hex.EncodeToString(zpayInvoice.PaymentHash[:]),
	}, nil
}

type PaymentType uint

const (
	All      PaymentType = iota
	Incoming PaymentType = iota + 1
	Outgoing PaymentType = iota + 2
)

func (l *LDK) Payments(paymentType PaymentType) ([]ldk_node.PaymentDetails, error) {
	node, err := l.getNode()
	if err != nil {
		return nil, err
	}
	return filterPaymentsByType(node.ListPayments(), paymentType)
}

func filterPaymentsByType(payments []ldk_node.PaymentDetails, paymentType PaymentType) ([]ldk_node.PaymentDetails, error) {
	filteredPayments := make([]ldk_node.PaymentDetails, 0, len(payments))
	for _, payment := range payments {
		switch payment.Kind.(type) {
		case *ldk_node.PaymentKindBolt11,
			ldk_node.PaymentKindBolt11Jit,
			*ldk_node.PaymentKindBolt11Jit,
			ldk_node.PaymentKindBolt11:
			if payment.Status == ldk_node.PaymentStatusPending {
				continue
			}
			if payment.Status == ldk_node.PaymentStatusFailed {
				continue
			}
		}

		switch paymentType {
		case Incoming:
			if payment.Direction == ldk_node.PaymentDirectionInbound {
				filteredPayments = append(filteredPayments, payment)
			}
		case Outgoing:
			if payment.Direction == ldk_node.PaymentDirectionOutbound {
				filteredPayments = append(filteredPayments, payment)
			}
		case All:
			filteredPayments = append(filteredPayments, payment)
		default:
			return nil, fmt.Errorf("unknown payment type: %+v", paymentType)
		}
	}

	return filteredPayments, nil
}
