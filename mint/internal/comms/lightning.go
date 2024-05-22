package comms

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	FAKE_WALLET  = "FakeWallet"
	LND_WALLET   = "LndGrpcWallet"
	LND_HOST     = "LND_GRPC_HOST"
	LND_TLS_CERT = "LND_TLS_CERT"
	LND_MACAROON = "LND_MACAROON"
)

type LightingComms struct {
	RpcClient *grpc.ClientConn
	Macaroon  string
}

func (l *LightingComms) RequestInvoice(amount int64) (*lnrpc.AddInvoiceResponse, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

	client := lnrpc.NewLightningClient(l.RpcClient)

	res, err := client.AddInvoice(ctx, &lnrpc.Invoice{Value: amount, Expiry: 3600})

	if err != nil {
		return nil, err
	}

	return res, nil

}
func (l *LightingComms) CheckIfInvoicePayed(hash string) (*lnrpc.Invoice, error) {

	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

	client := lnrpc.NewLightningClient(l.RpcClient)
	decodedHash, err := hex.DecodeString(hash)
	if err != nil {
        return nil, fmt.Errorf("hex.DecodeString: %w. hash: %s", err, hash ) 
	}

	rhash := lnrpc.PaymentHash{
		RHash: decodedHash,
	}

	invoice, err := client.LookupInvoice(ctx, &rhash)

	if err != nil {
		return nil, err
	}
	return invoice, nil
}

func (l *LightingComms) PayInvoice(invoice string) (*lnrpc.SendResponse, error) {

	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

	client := lnrpc.NewLightningClient(l.RpcClient)

	res, err := client.SendPaymentSync(ctx, &lnrpc.SendRequest{PaymentRequest: invoice})

	if err != nil {
		return nil, err
	}
	return res, nil
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

func (l *LightingComms) QueryPayment(invoice *zpay32.Invoice) (*lnrpc.QueryRoutesResponse, error) {

	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.Macaroon)

	client := lnrpc.NewLightningClient(l.RpcClient)

	routeHints := convert_route_hints(invoice.RouteHints)

	featureBits := getFeatureBits(invoice.Features)

	queryRoutes := lnrpc.QueryRoutesRequest{
		PubKey:            hex.EncodeToString(invoice.Destination.SerializeCompressed()),
		AmtMsat:           int64(*invoice.MilliSat),
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
	return res, nil
}

func SetupLightingComms() (*LightingComms, error) {
	host := os.Getenv(LND_HOST)
	if host == "" {
		return nil, fmt.Errorf("LND_HOST not available")
	}
	pem_cert := os.Getenv(LND_TLS_CERT)
	if pem_cert == "" {
		return nil, fmt.Errorf("LND_CERT_PATH not available")
	}

	certPool := x509.NewCertPool()
	appendOk := certPool.AppendCertsFromPEM([]byte(pem_cert))

	if !appendOk {
		log.Printf("x509.AppendCertsFromPEM(): failed")
		return nil, fmt.Errorf("x509.AppendCertsFromPEM(): failed")
	}

	certFile := credentials.NewClientTLSFromCert(certPool, "")

	tlsDialOption := grpc.WithTransportCredentials(certFile)

	dialOpts := []grpc.DialOption{
		tlsDialOption,
	}

	clientConn, err := grpc.Dial(host, dialOpts...)

	if err != nil {
		return nil, err
	}

	macaroon := os.Getenv(LND_MACAROON)

	if macaroon == "" {
		return nil, fmt.Errorf("LND_MACAROON_PATH not available")
	}

	return &LightingComms{Macaroon: macaroon, RpcClient: clientConn}, nil
}
