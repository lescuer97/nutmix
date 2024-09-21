package lightning

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"crypto/x509"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type LndGrpcWallet struct {
	Network    chaincfg.Params
	grpcClient *grpc.ClientConn
	macaroon   string
}

func (l *LndGrpcWallet) SetupGrpc(host string, macaroon string, tlsCrt string) error {
	if host == "" {
		return fmt.Errorf("LND_HOST not available")
	}

	if tlsCrt == "" {
		return fmt.Errorf("LND_CERT_PATH not available")
	}

	certPool := x509.NewCertPool()
	appendOk := certPool.AppendCertsFromPEM([]byte(tlsCrt))

	if !appendOk {
		return fmt.Errorf("x509.AppendCertsFromPEM(): failed")
	}

	certFile := credentials.NewClientTLSFromCert(certPool, "")

	tlsDialOption := grpc.WithTransportCredentials(certFile)

	dialOpts := []grpc.DialOption{
		tlsDialOption,
	}

	clientConn, err := grpc.Dial(host, dialOpts...)

	if err != nil {
		return err
	}

	if macaroon == "" {
		return fmt.Errorf("LND_MACAROON_PATH not available")
	}

	l.macaroon = macaroon
	l.grpcClient = clientConn
	return nil
}

func (l *LndGrpcWallet) lndGrpcPayInvoice(invoice string, feeReserve uint64, lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)
	client := lnrpc.NewLightningClient(l.grpcClient)

	fixedLimit := lnrpc.FeeLimit_Fixed{
		Fixed: int64(feeReserve),
	}

	feeLimit := lnrpc.FeeLimit{
		Limit: &fixedLimit,
	}

	sendRequest := lnrpc.SendRequest{PaymentRequest: invoice, AllowSelfPayment: true, FeeLimit: &feeLimit}

	res, err := client.SendPaymentSync(ctx, &sendRequest)

	if err != nil {
		return err
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaymentError = fmt.Errorf(res.PaymentError)
	lightningResponse.PaidFeeSat = res.PaymentRoute.TotalFeesMsat / 1000

	return nil

}
func (l *LndGrpcWallet) lndGrpcPayPartialInvoice(invoice string,
	zpayInvoice *zpay32.Invoice,
	feeReserve uint64,
	amount_sat uint64,
	lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	fixedLimit := lnrpc.FeeLimit_Fixed{
		Fixed: int64(feeReserve),
	}
	// zpayInvoice.PaymentHash

	feeLimit := lnrpc.FeeLimit{
		Limit: &fixedLimit,
	}
	queryRoutes := lnrpc.QueryRoutesRequest{
		PubKey:            hex.EncodeToString(zpayInvoice.Destination.SerializeCompressed()),
		UseMissionControl: true,
		Amt:               int64(amount_sat),
		FeeLimit:          &feeLimit,
	}

	// check for query hops
	queryResponse, err := client.QueryRoutes(ctx, &queryRoutes)
	if err != nil {
		return fmt.Errorf("client.QueryRoutes(ctx, &queryRoutes) %w", err)
	}

	firstRoute := queryResponse.Routes[0]

	if firstRoute == nil {
		return fmt.Errorf("No Route found %w", err)
	}

	lastHop := firstRoute.Hops[len(firstRoute.Hops)-1]

	totalMilisats := int64(*zpayInvoice.MilliSat)

	mppRecord := lnrpc.MPPRecord{
		TotalAmtMsat: totalMilisats,
		PaymentAddr:  zpayInvoice.PaymentAddr[:],
	}
	lastHop.MppRecord = &mppRecord
	firstRoute.Hops[len(firstRoute.Hops)-1] = lastHop

	streamerClient := routerrpc.NewRouterClient(l.grpcClient)

	sendRequest := routerrpc.SendToRouteRequest{PaymentHash: zpayInvoice.PaymentHash[:], Route: firstRoute, SkipTempErr: true}

	res, err := streamerClient.SendToRouteV2(ctx, &sendRequest)

	if err != nil {
		return fmt.Errorf("client.SendPaymentV2(ctx, &sendRequest) %w", err)
	}

	for {
		switch res.Status {
		case lnrpc.HTLCAttempt_FAILED:
			return fmt.Errorf("PaymentFailed  %+v", res.GetFailure())
		case lnrpc.HTLCAttempt_SUCCEEDED:
			lightningResponse.PaymentRequest = invoice
			lightningResponse.Preimage = hex.EncodeToString(res.Preimage)
			lightningResponse.PaymentError = errors.New("")
			lightningResponse.PaidFeeSat = res.Route.TotalAmt
			return nil
		default:
			continue

		}
	}

}

func (l LndGrpcWallet) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error) {
	var invoiceRes PaymentResponse
	if mpp {
		err := l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
		}
	} else {
		err := l.lndGrpcPayInvoice(invoice, feeReserve, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
		}
	}

	return invoiceRes, nil
}

func (l LndGrpcWallet) CheckPayed(quote string) (cashu.ACTION_STATE, string, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	decodedHash, err := hex.DecodeString(quote)
	if err != nil {
		return cashu.UNPAID, "", fmt.Errorf("hex.DecodeString: %w. hash: %s", err, quote)
	}

	rhash := lnrpc.PaymentHash{
		RHash: decodedHash,
	}

	invoice, err := client.LookupInvoice(ctx, &rhash)

	if err != nil {
		return cashu.UNPAID, "", err
	}
	switch {
	case invoice.State == lnrpc.Invoice_SETTLED:
		return cashu.PAID, hex.EncodeToString(invoice.RPreimage), nil

	case invoice.State == lnrpc.Invoice_OPEN:
		return cashu.UNPAID, hex.EncodeToString(invoice.RPreimage), nil

	}
	return cashu.UNPAID, "", nil
}

func convert_route_hints(routes [][]zpay32.HopHint) []*lnrpc.RouteHint {
	routehints := []*lnrpc.RouteHint{}
	for _, route := range routes {
		var hopHints []*lnrpc.HopHint
		for _, hint := range route {
			hophint := lnrpc.HopHint{
				NodeId:                    hex.EncodeToString(hint.NodeID.SerializeCompressed()),
				ChanId:                    hint.ChannelID,
				FeeBaseMsat:               hint.FeeBaseMSat,
				FeeProportionalMillionths: hint.FeeProportionalMillionths,
				CltvExpiryDelta:           uint32(hint.CLTVExpiryDelta),
			}
			hopHints = append(hopHints, &hophint)
		}

		routehints = append(routehints, &lnrpc.RouteHint{
			HopHints: *&hopHints,
		})
	}
	return routehints
}

func getFeatureBits(features *lnwire.FeatureVector) []lnrpc.FeatureBit {
	invoiceFeatures := features.Features()
	featureBits := make([]lnrpc.FeatureBit, len(invoiceFeatures))

	for k := range invoiceFeatures {
		feature := lnrpc.FeatureBit(int32(k))
		featureBits = append(featureBits, feature)
	}
	return featureBits
}

func (l LndGrpcWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	routeHints := convert_route_hints(zpayInvoice.RouteHints)

	featureBits := getFeatureBits(zpayInvoice.Features)

	queryRoutes := lnrpc.QueryRoutesRequest{
		PubKey:            hex.EncodeToString(zpayInvoice.Destination.SerializeCompressed()),
		RouteHints:        routeHints,
		DestFeatures:      featureBits,
		UseMissionControl: true,
		Amt:               int64(amount_sat),
	}

	res, err := client.QueryRoutes(ctx, &queryRoutes)

	if err != nil {
		return 1, err
	}
	if res == nil {
		return 1, fmt.Errorf("No routes found")
	}

	fee := GetAverageRouteFee(res.Routes) / 1000

	fmt.Println("Res.Fees: ", GetAverageRouteFee(res.Routes))

	return fee, nil
}

func (l LndGrpcWallet) RequestInvoice(amount int64) (InvoiceResponse, error) {
	var response InvoiceResponse
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	// Expiry time is 15 minutes
	res, err := client.AddInvoice(ctx, &lnrpc.Invoice{Value: amount, Expiry: 900})

	if err != nil {
		return response, err
	}

	response.Rhash = hex.EncodeToString(res.RHash)
	response.PaymentRequest = res.PaymentRequest

	return response, nil
}

func (l LndGrpcWallet) WalletBalance() (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	channelRequest := lnrpc.ChannelBalanceRequest{}

	balance, err := client.ChannelBalance(ctx, &channelRequest)

	if err != nil {
		return 0, err
	}
	return balance.LocalBalance.GetMsat(), nil
}

func (f LndGrpcWallet) LightningType() Backend {
	return LNDGRPC
}

func (f LndGrpcWallet) GetNetwork() *chaincfg.Params {
	return &f.Network
}
func (f LndGrpcWallet) ActiveMPP() bool {
	return true
}
