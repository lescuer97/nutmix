package lightning

import (
	"context"
	"net"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mppAmountLightningServer struct {
	lnrpc.UnimplementedLightningServer
	queryRoutesRequest *lnrpc.QueryRoutesRequest
}

func (s *mppAmountLightningServer) QueryRoutes(_ context.Context, request *lnrpc.QueryRoutesRequest) (*lnrpc.QueryRoutesResponse, error) {
	s.queryRoutesRequest = request
	return &lnrpc.QueryRoutesResponse{Routes: []*lnrpc.Route{{Hops: []*lnrpc.Hop{{}}}}}, nil //nolint:exhaustruct
}

type mppAmountRouterServer struct {
	routerrpc.UnimplementedRouterServer
}

func (s *mppAmountRouterServer) SendToRouteV2(_ context.Context, _ *routerrpc.SendToRouteRequest) (*lnrpc.HTLCAttempt, error) {
	return &lnrpc.HTLCAttempt{ //nolint:exhaustruct
		Status:   lnrpc.HTLCAttempt_SUCCEEDED,
		Route:    &lnrpc.Route{}, //nolint:exhaustruct
		Preimage: make([]byte, 32),
	}, nil
}

func TestLNDMPPUsesPrincipalAmountAndMsatQueryUnits(t *testing.T) {
	invoiceString, err := CreateMockInvoice(cashu.NewAmount(cashu.Sat, 100), "mpp amount", chaincfg.MainNetParams, cashu.ExpiryTimeMinUnit(15))
	if err != nil {
		t.Fatalf("CreateMockInvoice: %v", err)
	}
	invoice, err := zpay32.Decode(invoiceString, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("zpay32.Decode: %v", err)
	}

	tests := []struct {
		name         string
		amount       cashu.Amount
		wantAmtMsat  int64
		feeReserve   cashu.Amount
		wantFeeLimit int64
	}{
		{name: "sat principal", amount: cashu.NewAmount(cashu.Sat, 12), wantAmtMsat: 12000, feeReserve: cashu.NewAmount(cashu.Sat, 2), wantFeeLimit: 2},
		{name: "msat principal", amount: cashu.NewAmount(cashu.Msat, 3001), wantAmtMsat: 3001, feeReserve: cashu.NewAmount(cashu.Sat, 2), wantFeeLimit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer()
			lightningServer := &mppAmountLightningServer{} //nolint:exhaustruct
			lnrpc.RegisterLightningServer(server, lightningServer)
			routerrpc.RegisterRouterServer(server, &mppAmountRouterServer{}) //nolint:exhaustruct
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)

			conn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("grpc.NewClient: %v", err)
			}
			t.Cleanup(func() { _ = conn.Close() })

			response, err := (LndGrpcWallet{grpcClient: conn}).PayInvoice( //nolint:exhaustruct
				cashu.MeltRequestDB{Request: invoiceString}, invoice, tt.feeReserve, true, tt.amount, //nolint:exhaustruct
			)
			if err != nil {
				t.Fatalf("PayInvoice: %v", err)
			}
			if response.PaymentState != SETTLED {
				t.Errorf("PaymentState = %v, want %v", response.PaymentState, SETTLED)
			}
			if got := lightningServer.queryRoutesRequest.GetAmtMsat(); got != tt.wantAmtMsat {
				t.Errorf("QueryRoutes AmtMsat = %d, want %d", got, tt.wantAmtMsat)
			}
			if got := lightningServer.queryRoutesRequest.GetFeeLimit().GetFixed(); got != tt.wantFeeLimit {
				t.Errorf("QueryRoutes fee limit = %d, want %d", got, tt.wantFeeLimit)
			}
		})
	}
}
