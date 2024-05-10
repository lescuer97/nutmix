package comms

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	FAKE_WALLET          = "FakeWallet"
	LND_WALLET          = "LndGrpcWallet"
	LND_HOST          = "LND_GRPC_HOST"
	LND_CERT_PATH     = "LND_CERT_PATH"
	LND_MACAROON_PATH = "LND_MACAROON_PATH"
)

type LightingComms struct {
    RpcClient *grpc.ClientConn
    Macaroon string
}


func (l *LightingComms) RequestInvoice(amount int64) (*lnrpc.AddInvoiceResponse, error) {
    ctx := metadata.AppendToOutgoingContext(context.Background(),  "macaroon", l.Macaroon)

    client := lnrpc.NewLightningClient(l.RpcClient)

    res , err := client.AddInvoice(ctx, &lnrpc.Invoice{Value: amount, Expiry: 3600})

    if err != nil {
        return nil, err
    }

    return res, nil

}
func (l *LightingComms) CheckIfInvoicePayed(hash string) (*lnrpc.Invoice, error) {

    ctx := metadata.AppendToOutgoingContext(context.Background(),  "macaroon", l.Macaroon)

    client := lnrpc.NewLightningClient(l.RpcClient)
    decodedHash, err := hex.DecodeString(hash)
    if err != nil {
        return nil, err
    }

    rhash := lnrpc.PaymentHash{
        RHash: decodedHash,
    }

    invoice , err :=  client.LookupInvoice(ctx, &rhash )


    if err != nil {
        return nil, err
    }
    return invoice, nil
}

func (l *LightingComms) PayInvoice(invoice string) (*lnrpc.SendResponse, error) {

    ctx := metadata.AppendToOutgoingContext(context.Background(),  "macaroon", l.Macaroon)

    client := lnrpc.NewLightningClient(l.RpcClient)

    res, err := client.SendPaymentSync(ctx, &lnrpc.SendRequest{PaymentRequest: invoice})

    if err != nil {
        return nil, err
    }
    return res, nil
}

func (l *LightingComms) QueryPayment(invoice *zpay32.Invoice) (*lnrpc.QueryRoutesResponse, error) {

    ctx := metadata.AppendToOutgoingContext(context.Background(),  "macaroon", l.Macaroon)

    client := lnrpc.NewLightningClient(l.RpcClient)

    queryRoutes := lnrpc.QueryRoutesRequest {
        PubKey: hex.EncodeToString(invoice.Destination.SerializeCompressed()),
        AmtMsat: int64(*invoice.MilliSat),
    }

    res, err := client.QueryRoutes(ctx, &queryRoutes)
    
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
	certPath := os.Getenv(LND_CERT_PATH)
	if certPath == "" {
		return nil, fmt.Errorf("LND_CERT_PATH not available")
	}
	macaroonPath := os.Getenv(LND_MACAROON_PATH)
	if macaroonPath == "" {
		return nil, fmt.Errorf("LND_MACAROON_PATH not available")
	}

    macaroonBytes, err := os.ReadFile(macaroonPath)

	if err != nil {
		return nil, fmt.Errorf("error reading macaroon: os.ReadFile %v", err)
	}

	macaroonHex := hex.EncodeToString(macaroonBytes)

    certFile, err := credentials.NewClientTLSFromFile(certPath, "")

    if err != nil {
        return nil, err
    }

    tlsDialOption := grpc.WithTransportCredentials(certFile)

    
    dialOpts := []grpc.DialOption{
        tlsDialOption,
    }
    
    clientConn, err := grpc.Dial(host, dialOpts...)

    if err != nil {
        return nil, err
    }

    return &LightingComms{Macaroon: macaroonHex, RpcClient: clientConn }, nil
}

