package admin

import (
	"errors"
	"fmt"

	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
	m "github.com/lescuer97/nutmix/internal/mint"
)

const LiquidCoinType = 1776

var (
	ErrAmountUnderSendLimit = errors.New("Amount under swap limit")
	ErrAmountOverSendLimit  = errors.New("Amount over swap limit")
)

// sending to liquid steps
// prepare and send to local liquid address
// after confirmation send to liquid address provided by the user
func LightningToLiquidSwap(amount uint64, sdk *breez_sdk_liquid.BindingLiquidSdk) (breez_sdk_liquid.ReceivePaymentResponse, error) {
	var receivePayment breez_sdk_liquid.ReceivePaymentResponse
	// Fetch the lightning Receive limits
	currentLimits, err := sdk.FetchLightningLimits()

	if err != nil {
		return receivePayment, fmt.Errorf("sdk.FetchLightningLimits(); %w", err)
	}

	switch {
	case amount < currentLimits.Send.MinSat:
		return receivePayment, ErrAmountUnderSendLimit
	case amount > currentLimits.Send.MaxSat:
		return receivePayment, ErrAmountOverSendLimit
	}

	// Set the invoice amount you wish the payer to send, which should be within the above limits
	prepareRequest := breez_sdk_liquid.PrepareReceiveRequest{
		PaymentMethod:  breez_sdk_liquid.PaymentMethodLightning,
		PayerAmountSat: &amount,
	}

	prepareResponse, err := sdk.PrepareReceivePayment(prepareRequest)

	if err != nil {
		return receivePayment, fmt.Errorf("sdk.PrepareReceivePayment(prepareRequest). %w", err)
	}

	req := breez_sdk_liquid.ReceivePaymentRequest{
		PrepareResponse: prepareResponse,
	}
	res, err := sdk.ReceivePayment(req)
	if err != nil {
		// If the fees are acceptable, continue to create the Receive Payment
		return receivePayment, fmt.Errorf("sdk.ReceivePayment(req). %w", err)
	}

	return res, nil
}

func CheckLightningToLiquidSwap(swapId string) error {
	return nil
}

func SdkLiquidToRemoteLiquidAddress(mint m.Mint) error {
	return nil
}

func CheckLiquidToLightningSwap(swapId string) error {
	return nil
}
