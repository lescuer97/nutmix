package utils

import (
	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
	"github.com/btcsuite/btcd/chaincfg"
)

type SwapState string

const WaitingBoltzTXConfirmations SwapState = "WaitingBoltzTXConfirmations"
const BoltzWaitingPayment SwapState = "BoltzWaitingPayment"
const WaitingUserConfirmation SwapState = "WaitingUserConfirmation"

const MintWaitingPaymentRecv SwapState = "MintWaitingPaymentRecv"

const Finished SwapState = "Finished"
const Expired SwapState = "Expired"
const LightnigPaymentFail SwapState = "LightnigPaymentFail"
const UnknownProblem SwapState = "UnknownProblem"

type SwapType string

const LiquidityOut SwapType = "LiquidityOut"
const LiquidityIn SwapType = "LiquidityIn"

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
	Amount      uint64    `json"amount"`
	Id          string    `json"id"`
	Destination string    `json"destination"`
	State       SwapState `json"state"`
	Type        SwapType  `json"type"`
	Expiration  uint64    `json"expiration"`
}
