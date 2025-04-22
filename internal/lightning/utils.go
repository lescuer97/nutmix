package lightning

import (
	"math"

	"github.com/lightningnetwork/lnd/lnrpc"
)

const (
	MAINNET  = "mainnet"
	REGTEST  = "regtest"
	TESTNET  = "testnet"
	TESTNET3 = "testnet3"
	SIGNET   = "signet"
)

const MinimumLightningFee float64 = 0.01

func GetAverageRouteFee(routes []*lnrpc.Route) uint64 {
	var fees uint64
	var amount_routes uint64

	for _, route := range routes {
		fees += uint64(route.TotalFeesMsat)
		amount_routes += 1
	}
	return fees / amount_routes
}

func GetFeeReserve(invoiceSatAmount uint64, queriedFee uint64) uint64 {
	invoiceMinFee := float64(invoiceSatAmount) * MinimumLightningFee

	fee := uint64(math.Max(invoiceMinFee, float64(queriedFee)))
	return fee
}
