package utils

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

type SwapRequest struct {
	Amount      uint      `json"amount"`
	Id          string    `json"id"`
	Destination string    `json"destination"`
	State       SwapState `json"state"`
	Type        SwapType  `json"type"`
}
