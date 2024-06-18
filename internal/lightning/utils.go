package lightning

import "github.com/lightningnetwork/lnd/lnrpc"

func GetAverageRouteFee(routes []*lnrpc.Route) uint64 {
	var fees uint64
	var amount_routes uint64

	for _, route := range routes {
		fees += uint64(route.TotalFeesMsat)
		amount_routes += 1
	}
	return fees / amount_routes
}
