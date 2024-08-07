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
	"log"
	"net/http"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/lnrpc"
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

type LightningInvoiceRequest struct {
	Amount int64  `json:"amount"`
	Unit   string `json:"unit"`
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
}

func (l *LightingComms) LnbitsInvoiceRequest(method string, endpoint string, reqBody any, responseType any) error {
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
		err := l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
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
		var paymentStatus struct {
			Paid     bool   `json:"paid"`
			Pending  bool   `json:"pending"`
			Preimage string `json:"preimage"`
		}
		err := l.LnbitsInvoiceRequest("GET", "/api/v1/payments/"+quote, nil, &paymentStatus)
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

func (l *LightingComms) PayInvoice(invoice string, feeReserve uint64) (*LightningPaymentResponse, error) {
	var invoiceRes LightningPaymentResponse

	reqInvoice := LightningInvoiceRequest{
		Out:    true,
		Bolt11: invoice,
	}

	switch l.LightningBackend {
	case LNDGRPC:

		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

		fixedLimit := lnrpc.FeeLimit_Fixed{
			Fixed: int64(feeReserve),
		}

		feeLimit := lnrpc.FeeLimit{
			Limit: &fixedLimit,
		}
		sendRequest := lnrpc.SendRequest{PaymentRequest: invoice, AllowSelfPayment: true, FeeLimit: &feeLimit}

		res, err := client.SendPaymentSync(ctx, &sendRequest)

		if err != nil {
			return nil, err
		}

		invoiceRes.PaymentRequest = invoice
		invoiceRes.Preimage = hex.EncodeToString(res.PaymentPreimage)
		invoiceRes.PaymentError = fmt.Errorf(res.PaymentError)

		return &invoiceRes, nil

	case LNBITS:
		var lnbitsInvoice struct {
			PaymentHash    string `json:"payment_hash"`
			PaymentRequest string `json:"payment_request"`
		}
		err := l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice)
		if err != nil {
			return &invoiceRes, fmt.Errorf("json.Marshal: %w", err)
		}

		invoiceRes.PaymentRequest = lnbitsInvoice.PaymentRequest
		invoiceRes.Rhash = lnbitsInvoice.PaymentHash
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
}

func (l *LightingComms) QueryPayment(zpayInvoice *zpay32.Invoice, invoice string) (*QueryRoutesResponse, error) {
	var queryResponse QueryRoutesResponse

	switch l.LightningBackend {
	case LNDGRPC:

		ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

		client := lnrpc.NewLightningClient(l.LndRpcClient)

		routeHints := convert_route_hints(zpayInvoice.RouteHints)

		featureBits := getFeatureBits(zpayInvoice.Features)

		queryRoutes := lnrpc.QueryRoutesRequest{
			PubKey:            hex.EncodeToString(zpayInvoice.Destination.SerializeCompressed()),
			AmtMsat:           int64(*zpayInvoice.MilliSat),
			RouteHints:        routeHints,
			DestFeatures:      featureBits,
			UseMissionControl: true,
		}

		res, err := client.QueryRoutes(ctx, &queryRoutes)

		if res == nil {
			return nil, fmt.Errorf("No routes found")
		}

		if err != nil {
			return nil, err
		}
		fee := lightning.GetAverageRouteFee(res.Routes) / 1000
		queryResponse.FeeReserve = fee

		return &queryResponse, nil

	case LNBITS:
		invoiceString := "/api/v1/payments/fee-reserve" + "?" + `invoice=` + invoice

		err := l.LnbitsInvoiceRequest("GET", invoiceString, nil, &queryResponse)
		queryResponse.FeeReserve = queryResponse.FeeReserve / 1000

		if err != nil {
			return nil, fmt.Errorf("json.Marshal: %w", err)
		}

		return &queryResponse, nil

	}
	return nil, fmt.Errorf("Something happened")

}

func SetupLightingComms(ctx context.Context) (*LightingComms, error) {
	usedLightningBackend := ctx.Value("MINT_LIGHTNING_BACKEND")

	var lightningComs LightingComms
	switch usedLightningBackend {
	case LND_WALLET:
		err := setupLndRpcComms(ctx, &lightningComs)
		lightningComs.LightningBackend = LNDGRPC

		if err != nil {
			return nil, fmt.Errorf("Could not setup LND comms: %w", err)
		}

	case LNBITS_WALLET:

		mint_key := ctx.Value(MINT_LNBITS_KEY).(string)

		if mint_key == "" {
			return nil, fmt.Errorf("MINT_LNBITS_KEY not available")
		}
		mint_endpoint := ctx.Value(MINT_LNBITS_ENDPOINT).(string)
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

func setupLndRpcComms(ctx context.Context, lightningComs *LightingComms) error {
	host := ctx.Value(LND_HOST).(string)
	if host == "" {
		return fmt.Errorf("LND_HOST not available")
	}
	pem_cert := ctx.Value(LND_TLS_CERT).(string)

	if pem_cert == "" {
		return fmt.Errorf("LND_CERT_PATH not available")
	}

	certPool := x509.NewCertPool()
	appendOk := certPool.AppendCertsFromPEM([]byte(pem_cert))

	if !appendOk {
		log.Printf("x509.AppendCertsFromPEM(): failed")
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

	macaroon := ctx.Value(LND_MACAROON).(string)

	if macaroon == "" {
		return fmt.Errorf("LND_MACAROON_PATH not available")
	}
	lightningComs.LndRpcClient = clientConn
	lightningComs.Macaroon = macaroon

	return nil
}
