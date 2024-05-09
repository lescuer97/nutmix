package lightning

import "github.com/lightningnetwork/lnd/lnrpc"

func GetAverageRouteFee(routes []*lnrpc.Route) int64 {

    var fees int64
    var amount_routes int

    for _, route := range routes {
        fees += route.TotalFeesMsat
        amount_routes += 1
    }
    return fees / int64(amount_routes)
}
