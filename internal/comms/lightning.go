package comms

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	FAKE_WALLET          = "FakeWallet"
	LND_WALLET           = "LndGrpcWallet"
	LND_HOST             = "LND_GRPC_HOST"
	LND_TLS_CERT         = "LND_TLS_CERT"
	LND_MACAROON         = "LND_MACAROON"
	LNBITS_WALLET        = "LNbitsWallet"
	MINT_LNBITS_ENDPOINT = "MINT_LNBITS_ENDPOINT"
	MINT_LNBITS_KEY      = "MINT_LNBITS_KEY"
)

type LightningBackend uint

const LNDGRPC LightningBackend = iota + 1
const LNBITS LightningBackend = iota + 2

type LightingComms struct {
	LndRpcClient     *grpc.ClientConn
	Macaroon         string
	LightningBackend LightningBackend
	LnBitsData       LNBitsData
}

type LNBitsData struct {
	Key      string
	Endpoint string
}

type LNBitsDetailErrorData struct {
	Detail string
	Status string
}

type LNBitsPaymentStatusDetail struct {
	Memo    string
	Fee     int64
	Pending bool
}
type LNBitsPaymentStatus struct {
	Paid     bool   `json:"paid"`
	Pending  bool   `json:"pending"`
	Preimage string `json:"preimage"`
	Details  LNBitsPaymentStatusDetail
}

type LightningInvoiceRequest struct {
	Amount int64  `json:"amount"`
	Unit   string `json:"unit,omitempty"`
	Memo   string `json:"memo"`
	Out    bool   `json:"out"`
	Expiry int64  `json:"expiry"`
	Bolt11 string `json:"bolt11"`
}

type LightningInvoiceResponse struct {
	PaymentRequest string
	Rhash          string
}

type LightningPaymentResponse struct {
	Preimage       string
	PaymentError   error
	PaymentRequest string
	Rhash          string
	PaidFeeSat     int64
}

type LightingCommsData struct {
	MINT_LIGHTNING_BACKEND string
	LND_GRPC_HOST          string
	LND_TLS_CERT           string
	LND_MACAROON           string

	MINT_LNBITS_ENDPOINT string
	MINT_LNBITS_KEY      string
}

func (l *LightingComms) LnbitsRequest(method string, endpoint string, reqBody any, responseType any) error {
	client := &http.Client{}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	b := bytes.NewBuffer(jsonBytes)

	req, err := http.NewRequest(method, l.LnBitsData.Endpoint+endpoint, b)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("X-Api-Key", l.LnBitsData.Key)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(req): %w", err)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %w", err)
	}

	detailBody := LNBitsDetailErrorData{}
	err = json.Unmarshal(body, &detailBody)
	if err != nil {
		return fmt.Errorf("json.Unmarshal(detailBody): %w", err)
	}

	switch {
	case detailBody.Status == "failed":
		return fmt.Errorf("LNBITS payment failed %+v. Request Body %+v", detailBody, reqBody)

	case detailBody.Detail == "Payment does not exist.":
	case len(detailBody.Detail) > 0:
		return fmt.Errorf("LNBITS Unknown error %+v. Request Body %+v", detailBody, reqBody)
	}

	err = json.Unmarshal(body, &responseType)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	return nil

}

func (l *LightingComms) RequestInvoice(amount int64) (LightningInvoiceResponse, error) {
	var invoiceRes LightningInvoiceResponse
	reqInvoice := LightningInvoiceRequest{
		Amount: amount,
		Unit:   cashu.Sat.String(),
		Memo:   "",
		Out:    false,
		Expiry: 900,
	}
	switch l.LightningBackend {
	case LNDGRPC:
		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

		// Expiry time is 15 minutes
		res, err := client.AddInvoice(ctx, &lnrpc.Invoice{Value: reqInvoice.Amount, Expiry: reqInvoice.Expiry})

		if err != nil {
			return invoiceRes, err
		}

		invoiceRes.Rhash = hex.EncodeToString(res.RHash)
		invoiceRes.PaymentRequest = res.PaymentRequest

	case LNBITS:
		var lnbitsInvoice struct {
			PaymentHash    string `json:"payment_hash"`
			PaymentRequest string `json:"payment_request"`
		}
		err := l.LnbitsRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
		if err != nil {
			return invoiceRes, fmt.Errorf("json.Marshal: %w", err)
		}

		invoiceRes.PaymentRequest = lnbitsInvoice.PaymentRequest
		invoiceRes.Rhash = lnbitsInvoice.PaymentHash

		return invoiceRes, nil
	}

	return invoiceRes, nil
}

func (l *LightingComms) CheckIfInvoicePayed(quote string) (cashu.ACTION_STATE, string, error) {
	switch l.LightningBackend {
	case LNDGRPC:

		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

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
	case LNBITS:
		var paymentStatus LNBitsPaymentStatus

		err := l.LnbitsRequest("GET", "/api/v1/payments/"+quote, nil, &paymentStatus)
		if err != nil {
			return cashu.UNPAID, "", fmt.Errorf("json.Marshal: %w", err)
		}

		switch {
		case paymentStatus.Paid:
			return cashu.PAID, paymentStatus.Preimage, nil
		}

	}
	return cashu.UNPAID, "", nil

}

// return in milisat
func (l *LightingComms) WalletBalance() (uint64, error) {
	switch l.LightningBackend {
	case LNDGRPC:

		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

		channelRequest := lnrpc.ChannelBalanceRequest{}

		balance, err := client.ChannelBalance(ctx, &channelRequest)

		if err != nil {
			return 0, err
		}
		return balance.LocalBalance.GetMsat(), nil

	case LNBITS:
		var channelBalance struct {
			Id      string `json:"id"`
			Name    string `json:"name"`
			Balance int    `json:"balance"`
		}
		err := l.LnbitsRequest("GET", "/api/v1/wallet", nil, &channelBalance)
		if err != nil {
			return 0, fmt.Errorf("l.LnbitsInvoiceRequest: %w", err)
		}

		return uint64(channelBalance.Balance), nil

	}
	return 0, fmt.Errorf("Incorrent lightning backend")

}

func (l *LightingComms) lndGrpcPayInvoice(invoice string, feeReserve uint64, lightningResponse *LightningPaymentResponse) error {
	client := lnrpc.NewLightningClient(l.LndRpcClient)

	fixedLimit := lnrpc.FeeLimit_Fixed{
		Fixed: int64(feeReserve),
	}

	feeLimit := lnrpc.FeeLimit{
		Limit: &fixedLimit,
	}

	sendRequest := lnrpc.SendRequest{PaymentRequest: invoice, AllowSelfPayment: true, FeeLimit: &feeLimit}

	res, err := client.SendPaymentSync(context.Background(), &sendRequest)

	if err != nil {
		return err
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaymentError = fmt.Errorf(res.PaymentError)
	lightningResponse.PaidFeeSat = res.PaymentRoute.TotalFeesMsat / 1000

	return nil

}
func (l *LightingComms) lndGrpcPayPartialInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, amount_sat uint64, lightningResponse *LightningPaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

	client := lnrpc.NewLightningClient(l.LndRpcClient)

	fixedLimit := lnrpc.FeeLimit_Fixed{
		Fixed: int64(feeReserve),
	}

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

	streamerClient := routerrpc.NewRouterClient(l.LndRpcClient)

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

func (l *LightingComms) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (*LightningPaymentResponse, error) {
	var invoiceRes LightningPaymentResponse

	reqInvoice := LightningInvoiceRequest{
		Out:    true,
		Bolt11: invoice,
		Amount: int64(amount_sat),
	}

	switch l.LightningBackend {
	case LNDGRPC:
		if mpp {
			err := l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes)
			if err != nil {
				return &invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
			}
		} else {
			err := l.lndGrpcPayInvoice(invoice, feeReserve, &invoiceRes)
			if err != nil {
				return &invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
			}
		}

		return &invoiceRes, nil
	case LNBITS:
		var lnbitsInvoice struct {
			PaymentHash    string `json:"payment_hash"`
			PaymentRequest string `json:"payment_request"`
		}
		err := l.LnbitsRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
		if err != nil {
			return &invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
		}

		var paymentStatus LNBitsPaymentStatus

		// check invoice payment to get the preimage and fee
		err = l.LnbitsRequest("GET", "/api/v1/payments/"+lnbitsInvoice.PaymentHash, nil, &paymentStatus)
		if err != nil {

			return &invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("GET", "/api/v1/payments/"+lnbitsInvoice.PaymentHash, nil, &paymentStatus): %w`, err)
		}

		invoiceRes.PaymentRequest = lnbitsInvoice.PaymentRequest
		invoiceRes.Rhash = lnbitsInvoice.PaymentHash
		invoiceRes.Preimage = paymentStatus.Preimage
		invoiceRes.PaidFeeSat = int64(math.Abs(float64(paymentStatus.Details.Fee)))
		invoiceRes.PaymentError = errors.New("")

		return &invoiceRes, nil
	}

	return nil, nil
}

// Make route hints from zpay32 to lnrpc
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

type QueryRoutesResponse struct {
	FeeReserve uint64 `json:"fee_reserve"`
	Amount     uint64 `json:"amount"`
}

func (l *LightingComms) QueryPayment(zpayInvoice *zpay32.Invoice, invoice string, mpp bool, amount_sat uint64) (*QueryRoutesResponse, error) {
	var queryResponse QueryRoutesResponse

	switch l.LightningBackend {
	case LNDGRPC:

		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

		routeHints := convert_route_hints(zpayInvoice.RouteHints)

		featureBits := getFeatureBits(zpayInvoice.Features)

		queryRoutes := lnrpc.QueryRoutesRequest{
			PubKey: hex.EncodeToString(zpayInvoice.Destination.SerializeCompressed()),
			// AmtMsat:           int64(amount_sat),
			RouteHints:        routeHints,
			DestFeatures:      featureBits,
			UseMissionControl: true,
			Amt:               int64(amount_sat),
		}

		res, err := client.QueryRoutes(ctx, &queryRoutes)

		if err != nil {
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("No routes found")
		}

		fee := lightning.GetAverageRouteFee(res.Routes) / 1000
		queryResponse.FeeReserve = fee

		return &queryResponse, nil

	case LNBITS:
		invoiceString := "/api/v1/payments/fee-reserve" + "?" + `invoice=` + invoice

		err := l.LnbitsRequest("GET", invoiceString, nil, &queryResponse)
		queryResponse.FeeReserve = queryResponse.FeeReserve / 1000

		if err != nil {
			return nil, fmt.Errorf("json.Marshal: %w", err)
		}

		return &queryResponse, nil

	}
	return nil, fmt.Errorf("Something happened")

}

func SetupLightingComms(config LightingCommsData) (*LightingComms, error) {
	usedLightningBackend := config.MINT_LIGHTNING_BACKEND

	var lightningComs LightingComms
	switch usedLightningBackend {
	case LND_WALLET:
		err := SetupLndRpcComms(&lightningComs, config)
		lightningComs.LightningBackend = LNDGRPC

		if err != nil {
			return nil, fmt.Errorf("Could not setup LND comms: %w", err)
		}

	case LNBITS_WALLET:

		mint_key := config.MINT_LNBITS_KEY

		if mint_key == "" {
			return nil, fmt.Errorf("MINT_LNBITS_KEY not available")
		}
		mint_endpoint := config.MINT_LNBITS_ENDPOINT
		if mint_endpoint == "" {
			return nil, fmt.Errorf("MINT_LNBITS_ENDPOINT not available")
		}

		lightningComs.LightningBackend = LNBITS
		lightningComs.LnBitsData = LNBitsData{
			Key:      mint_key,
			Endpoint: mint_endpoint,
		}

	}

	return &lightningComs, nil

}

func SetupLndRpcComms(lightningComs *LightingComms, config LightingCommsData) error {
	host := config.LND_GRPC_HOST
	if host == "" {
		return fmt.Errorf("LND_HOST not available")
	}
	pem_cert := config.LND_TLS_CERT

	if pem_cert == "" {
		return fmt.Errorf("LND_CERT_PATH not available")
	}

	certPool := x509.NewCertPool()
	appendOk := certPool.AppendCertsFromPEM([]byte(pem_cert))

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

	macaroon := config.LND_MACAROON

	if macaroon == "" {
		return fmt.Errorf("LND_MACAROON_PATH not available")
	}
	lightningComs.LndRpcClient = clientConn
	lightningComs.Macaroon = macaroon

	return nil
}
