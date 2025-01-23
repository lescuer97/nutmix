package utils

import (
	"errors"

	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
	"github.com/btcsuite/btcd/chaincfg"
)

type SwapState string

const WaitingUserConfirmation SwapState = "WaitingUserConfirmation"
const MintWaitingPaymentRecv SwapState = "MintWaitingPaymentRecv"

const Finished SwapState = "Finished"
const Expired SwapState = "Expired"
const LightningPaymentFail SwapState = "LightningPaymentFail"
const LightningPaymentPending SwapState = "LightningPaymentPending"
const LightningPaymentExpired SwapState = "LightningPaymentExpired"
const UnknownProblem SwapState = "UnknownProblem"

var ErrAlreadyLNPaying = errors.New("already paying lightning invoice")

func (s SwapState) ToString() string {
	switch s {
	case WaitingUserConfirmation:
		return "Waiting for confirmation"
	case MintWaitingPaymentRecv:
		return "Waiting Receive Payment"
	case Finished:
		return string(Finished)
	case Expired:
		return string(Expired)
	case LightningPaymentFail:
		return "Failed lightning payment"
	case LightningPaymentExpired:
		return "Lighting payment expired"
	case LightningPaymentPending:
		return "Payment pending"
	case UnknownProblem:
		return "Unknown problem happened"
	}
	return ""
}

type SwapType string

const LiquidityOut SwapType = "LiquidityOut"
const LiquidityIn SwapType = "LiquidityIn"

func (s SwapType) ToString() string {

	switch s {
	case LiquidityOut:
		return "Out"
	case LiquidityIn:
		return "In"

	}
	return ""
}

func CanUseLiquidityManager(chain *chaincfg.Params) bool {
	switch chain {
	case &chaincfg.MainNetParams:
		return true
	case &chaincfg.TestNet3Params:
	default:
		return true
	}
	return false
}

func GetBreezLiquid(chain *chaincfg.Params) breez_sdk_liquid.LiquidNetwork {
	switch chain {
	case &chaincfg.MainNetParams:
		return breez_sdk_liquid.LiquidNetworkMainnet
	case &chaincfg.TestNet3Params:
	default:
		return breez_sdk_liquid.LiquidNetworkTestnet
	}

	return breez_sdk_liquid.LiquidNetworkTestnet
}

type LiquiditySwap struct {
	Amount           uint64    `json"amount"`
	Id               string    `json"id"`
	State            SwapState `json"state"`
	Type             SwapType  `json"type"`
	Expiration       uint64    `json"expiration"`
	LightningInvoice string    `db:"lightning_invoice"`
}
