package lightning

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

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
	grpcClient *grpc.ClientConn
	macaroon   string
	Network    chaincfg.Params
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

	clientConn, err := grpc.NewClient(host, dialOpts...)

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

func (l *LndGrpcWallet) lndGrpcPayInvoice(routerrpcClient routerrpc.RouterClient, invoiceString string, decodedInvoice *zpay32.Invoice, feeReserve uint64, lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	if decodedInvoice.MilliSat == nil {
		return fmt.Errorf("amount is not available for the invoice")
	}
	sendRequest := routerrpc.SendPaymentRequest{PaymentRequest: invoiceString, AllowSelfPayment: true}
	res, err := routerrpcClient.SendPaymentV2(ctx, &sendRequest)
	if err != nil {
		lightningResponse.PaymentState = FAILED
		return err
	}

	payment, err := res.Recv()
	if err != nil {
		return fmt.Errorf("res.Recv(). %w", err)
	}
	for {
		switch payment.Status {
		case lnrpc.Payment_IN_FLIGHT:
			lightningResponse.PaymentState = PENDING
		case lnrpc.Payment_INITIATED:
			lightningResponse.PaymentState = PENDING
		case lnrpc.Payment_FAILED:
			if payment.GetFailureReason() == lnrpc.PaymentFailureReason_FAILURE_REASON_NONE {
				continue
			}
			lightningResponse.PaymentState = FAILED
			return fmt.Errorf("PaymentFailed  %+v", payment.GetFailureReason().String())
		case lnrpc.Payment_SUCCEEDED:
			lightningResponse.PaymentRequest = invoiceString
			lightningResponse.PaymentState = SETTLED
			lightningResponse.Preimage = payment.GetPaymentPreimage()
			lightningResponse.PaidFeeSat = payment.FeeSat
			lightningResponse.PaymentState = SETTLED
			return nil
		default:
			continue

		}
	}
}

const MAX_AMOUNT_RETRIES = 50

func (l *LndGrpcWallet) lndGrpcPayPartialInvoice(
	routerrpcClient routerrpc.RouterClient,
	invoice string,
	zpayInvoice *zpay32.Invoice,
	feeReserve uint64,
	amount_sat uint64,
	lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	fixedLimit := lnrpc.FeeLimit_Fixed{
		Fixed: int64(feeReserve),
	}

	feeLimit := lnrpc.FeeLimit{
		Limit: &fixedLimit,
	}
	totalAttempts := 50
	var routes []*lnrpc.Route
	for i := 0; i < totalAttempts; i++ {

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

		routes = queryResponse.Routes

		if routes[0] == nil {
			log.Printf("No route found for lnd partial payment. Retrying")
			continue
		}

		totalMilisats := int64(*zpayInvoice.MilliSat)

		if zpayInvoice.PaymentAddr.IsNone() {
			return fmt.Errorf("could not find payment address in invoice")
		}
		paymentAddress := zpayInvoice.PaymentAddr.UnsafeFromSome()
		mppRecord := lnrpc.MPPRecord{
			TotalAmtMsat: totalMilisats,
			PaymentAddr:  paymentAddress[:],
		}

		routes[0].Hops[len(routes[0].Hops)-1].MppRecord = &mppRecord

		sendRequest := routerrpc.SendToRouteRequest{PaymentHash: zpayInvoice.PaymentHash[:], Route: routes[0], SkipTempErr: true}

		res, err := routerrpcClient.SendToRouteV2(ctx, &sendRequest)

		if err != nil {
			return fmt.Errorf("client.SendPaymentV2(ctx, &sendRequest) %w", err)
		}

		for {
			switch res.Status {
			case lnrpc.HTLCAttempt_IN_FLIGHT:
				lightningResponse.PaymentState = PENDING
			case lnrpc.HTLCAttempt_FAILED:
				if res.Failure.GetCode() == lnrpc.Failure_TEMPORARY_CHANNEL_FAILURE {
					failureIndex := res.Failure.GetFailureSourceIndex()
					failedSource := routes[0].Hops[failureIndex-1].PubKey
					failedDestination := routes[0].Hops[failureIndex].PubKey

					// TODO: change to use slog when refactor for slog
					log.Printf("partial payment attempt failed from %s to %s", failedSource, failedDestination)

					continue
				}
				lightningResponse.PaymentState = FAILED
				return fmt.Errorf("PaymentFailed  %+v", res.GetFailure())
			case lnrpc.HTLCAttempt_SUCCEEDED:
				lightningResponse.PaymentRequest = invoice
				lightningResponse.PaymentState = SETTLED
				lightningResponse.Preimage = hex.EncodeToString(res.Preimage)
				lightningResponse.PaidFeeSat = res.Route.TotalFeesMsat / 1000
				lightningResponse.PaymentState = SETTLED
				return nil
			default:
				continue

			}
		}
	}
	return fmt.Errorf("multi nut no route. %w", cashu.ErrPaymentNoRoute)

}

func (l LndGrpcWallet) PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var invoiceRes PaymentResponse

	routerClient := routerrpc.NewRouterClient(l.grpcClient)
	if mpp {
		err := l.lndGrpcPayPartialInvoice(routerClient, melt_quote.Request, zpayInvoice, feeReserve, amount.Amount, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
		}
	} else {
		err := l.lndGrpcPayInvoice(routerClient, melt_quote.Request, zpayInvoice, feeReserve, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
		}
	}
	invoiceRes.CheckingId = melt_quote.CheckingId

	return invoiceRes, nil
}

type LndPayStatus struct {
	Preimage string
	Fee      uint64
	Status   PaymentStatus
}

func (l LndGrpcWallet) getPaymentStatus(invoice *zpay32.Invoice) (LndPayStatus, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	routerClient := routerrpc.NewRouterClient(l.grpcClient)

	var payStatus LndPayStatus
	hash := invoice.PaymentHash[:]

	paymentstatusRequest := routerrpc.TrackPaymentRequest{PaymentHash: hash}

	res, err := routerClient.TrackPaymentV2(ctx, &paymentstatusRequest)

	if err != nil {
		return payStatus, err
	}

	for {
		payment, err := res.Recv()
		if err != nil {
			return payStatus, err
		}
		payStatus.Fee = uint64(payment.FeeSat)
		switch payment.Status {
		case lnrpc.Payment_IN_FLIGHT:
			payStatus.Status = PENDING
			return payStatus, nil
		case lnrpc.Payment_FAILED:
			payStatus.Status = FAILED
			return payStatus, nil
		case lnrpc.Payment_SUCCEEDED:
			payStatus.Status = SETTLED
			payStatus.Preimage = payment.PaymentPreimage
			return payStatus, nil
		case lnrpc.Payment_INITIATED:
			payStatus.Status = PENDING
			return payStatus, nil
		default:
			continue

		}
	}
}

func (l LndGrpcWallet) CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, uint64, error) {
	payStatus, err := l.getPaymentStatus(invoice)
	if err != nil {
		return FAILED, "", 0, fmt.Errorf(`l.getPaymentStatus(quote) %w`, err)
	}

	return payStatus.Status, payStatus.Preimage, payStatus.Fee, nil
}

func (l LndGrpcWallet) getInvoiceStatus(invoice *zpay32.Invoice) (*lnrpc.Invoice, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	hash := invoice.PaymentHash[:]

	rhash := lnrpc.PaymentHash{
		RHash: hash,
	}

	invoiceStat, err := client.LookupInvoice(ctx, &rhash)

	if err != nil {
		return nil, err
	}

	return invoiceStat, nil
}

func (l LndGrpcWallet) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	invoiceStatus, err := l.getInvoiceStatus(invoice)

	if err != nil {
		return FAILED, "", fmt.Errorf(`l.getInvoiceStatus(quote) %w`, err)
	}

	switch invoiceStatus.State {
	case lnrpc.Invoice_SETTLED:
		return SETTLED, hex.EncodeToString(invoiceStatus.RPreimage), nil
	case lnrpc.Invoice_CANCELED:
		return FAILED, hex.EncodeToString(invoiceStatus.RPreimage), nil

	case lnrpc.Invoice_OPEN:
		return PENDING, hex.EncodeToString(invoiceStatus.RPreimage), nil

	}
	return PENDING, "", nil
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
			HopHints: hopHints,
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

func (l LndGrpcWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	routeHints := convert_route_hints(zpayInvoice.RouteHints)

	featureBits := getFeatureBits(zpayInvoice.Features)

	queryRoutes := lnrpc.QueryRoutesRequest{
		PubKey:            hex.EncodeToString(zpayInvoice.Destination.SerializeCompressed()),
		RouteHints:        routeHints,
		DestFeatures:      featureBits,
		UseMissionControl: true,
		Amt:               int64(amount.Amount),
	}

	res, err := client.QueryRoutes(ctx, &queryRoutes)

	feesResponse := FeesResponse{}

	if err != nil {
		return feesResponse, err
	}
	if res == nil {
		return feesResponse, fmt.Errorf("no routes found")
	}

	fee := GetAverageRouteFee(res.Routes) / 1000

	fee = GetFeeReserve(amount.Amount, fee)

	hash := zpayInvoice.PaymentHash[:]

	feesResponse.Fees.Amount = fee
	feesResponse.AmountToSend.Amount = amount.Amount
	feesResponse.CheckingId = hex.EncodeToString(hash)
	return feesResponse, nil
}

func (l LndGrpcWallet) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
	var response InvoiceResponse
	supported := l.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit): %w", cashu.ErrUnitNotSupported)
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := lnrpc.NewLightningClient(l.grpcClient)

	err := amount.To(cashu.Sat)
	if err != nil {
		return response, fmt.Errorf(`amount.To(cashu.Sat) %w`, err)
	}

	Lndinvoice := lnrpc.Invoice{Value: int64(amount.Amount), Expiry: 900}
	if quote.Description != nil {
		Lndinvoice.Memo = *quote.Description
	}

	// Expiry time is 15 minutes
	res, err := client.AddInvoice(ctx, &Lndinvoice)

	if err != nil {
		return response, err
	}

	response.Rhash = hex.EncodeToString(res.RHash)
	response.PaymentRequest = res.PaymentRequest
	response.CheckingId = hex.EncodeToString(res.RHash)

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
func (f LndGrpcWallet) VerifyUnitSupport(unit cashu.Unit) bool {
	if unit == cashu.Sat {
		return true
	} else {
		return false
	}
}

func (f LndGrpcWallet) DescriptionSupport() bool {
	return true
}
