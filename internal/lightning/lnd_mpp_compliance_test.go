package lightning

import (
	"context"
	"errors"
	"math"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

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

// mppPartialPaymentAttempts mirrors the bounded retry budget used by
// lndGrpcPayPartialInvoice (totalAttempts). The route-safety tests assert on it
// so a regression that removes the bound is caught instead of hanging.
const mppPartialPaymentAttempts = 50

// mppRouteSafetyLightningServer serves a fixed set of routes for QueryRoutes and
// counts how many times it was queried.
type mppRouteSafetyLightningServer struct {
	routes []*lnrpc.Route
	lnrpc.UnimplementedLightningServer
	mu    sync.Mutex
	calls int
}

func (s *mppRouteSafetyLightningServer) QueryRoutes(_ context.Context, _ *lnrpc.QueryRoutesRequest) (*lnrpc.QueryRoutesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return &lnrpc.QueryRoutesResponse{Routes: s.routes}, nil //nolint:exhaustruct
}

func (s *mppRouteSafetyLightningServer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// mppRouteSafetyRouterServer replays a fixed list of SendToRouteV2 responses,
// repeating the final response once the list is exhausted. An empty list always
// succeeds.
type mppRouteSafetyRouterServer struct {
	responses []*lnrpc.HTLCAttempt
	routerrpc.UnimplementedRouterServer
	mu    sync.Mutex
	calls int
}

func (s *mppRouteSafetyRouterServer) SendToRouteV2(_ context.Context, _ *routerrpc.SendToRouteRequest) (*lnrpc.HTLCAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	call := s.calls
	s.calls++

	if len(s.responses) == 0 {
		return mppSucceededAttempt(), nil
	}
	if call >= len(s.responses) {
		call = len(s.responses) - 1
	}
	return s.responses[call], nil
}

func (s *mppRouteSafetyRouterServer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func mppSucceededAttempt() *lnrpc.HTLCAttempt {
	return &lnrpc.HTLCAttempt{ //nolint:exhaustruct
		Status:   lnrpc.HTLCAttempt_SUCCEEDED,
		Route:    &lnrpc.Route{}, //nolint:exhaustruct
		Preimage: make([]byte, 32),
	}
}

func mppTemporaryChannelFailure(sourceIndex uint32) *lnrpc.HTLCAttempt {
	return &lnrpc.HTLCAttempt{ //nolint:exhaustruct
		Status: lnrpc.HTLCAttempt_FAILED,
		Failure: &lnrpc.Failure{ //nolint:exhaustruct
			Code:               lnrpc.Failure_TEMPORARY_CHANNEL_FAILURE,
			FailureSourceIndex: sourceIndex,
		},
	}
}

func startMPPRouteSafetyBackend(t *testing.T, lightningServer *mppRouteSafetyLightningServer, routerServer *mppRouteSafetyRouterServer) *grpc.ClientConn {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	lnrpc.RegisterLightningServer(server, lightningServer)
	routerrpc.RegisterRouterServer(server, routerServer)
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

	return conn
}

// mppPayInvoiceWithinDeadline runs PayInvoice on its own goroutine so a
// regression that reintroduces an unbounded retry loop fails the test instead
// of hanging the whole package.
func mppPayInvoiceWithinDeadline(t *testing.T, conn *grpc.ClientConn, invoiceString string, invoice *zpay32.Invoice) (PaymentResponse, error) {
	t.Helper()

	var (
		response PaymentResponse
		payErr   error
	)

	done := make(chan struct{}, 1)
	go func() {
		response, payErr = (LndGrpcWallet{grpcClient: conn}).PayInvoice( //nolint:exhaustruct
			cashu.MeltRequestDB{Request: invoiceString}, invoice, cashu.NewAmount(cashu.Sat, 2), true, cashu.NewAmount(cashu.Sat, 12), //nolint:exhaustruct
		)
		done <- struct{}{}
	}()

	select {
	case <-done:
		return response, payErr
	case <-time.After(15 * time.Second):
		t.Fatalf("PayInvoice did not return within the deadline; retry loop is not bounded")
		return PaymentResponse{}, nil //nolint:exhaustruct
	}
}

// TestLNDMPPRouteFailureHopsBoundsIndex exercises the failure-index bounds check
// directly, covering index zero, the final-node index (== hop count), indexes
// beyond the hop count, nil hops and non-existent routes.
func TestLNDMPPRouteFailureHopsBoundsIndex(t *testing.T) {
	threeHopRoute := &lnrpc.Route{ //nolint:exhaustruct
		Hops: []*lnrpc.Hop{
			{PubKey: "hop0"}, //nolint:exhaustruct
			{PubKey: "hop1"}, //nolint:exhaustruct
			{PubKey: "hop2"}, //nolint:exhaustruct
		},
	}
	routeWithNilSourceHop := &lnrpc.Route{ //nolint:exhaustruct
		Hops: []*lnrpc.Hop{
			nil,
			{PubKey: "hop1"}, //nolint:exhaustruct
		},
	}
	routeWithNilDestinationHop := &lnrpc.Route{ //nolint:exhaustruct
		Hops: []*lnrpc.Hop{
			{PubKey: "hop0"}, //nolint:exhaustruct
			nil,
		},
	}

	tests := []struct {
		name            string
		route           *lnrpc.Route
		wantSource      string
		wantDestination string
		failureIndex    uint32
	}{
		{name: "nil route", route: nil, failureIndex: 1, wantSource: "", wantDestination: ""},
		{name: "empty hop list", route: &lnrpc.Route{}, failureIndex: 1, wantSource: "", wantDestination: ""}, //nolint:exhaustruct
		{name: "index zero is sender", route: threeHopRoute, failureIndex: 0, wantSource: "", wantDestination: "hop0"},
		{name: "first hop", route: threeHopRoute, failureIndex: 1, wantSource: "hop0", wantDestination: "hop1"},
		{name: "middle hop", route: threeHopRoute, failureIndex: 2, wantSource: "hop1", wantDestination: "hop2"},
		{name: "last hop equals hop count", route: threeHopRoute, failureIndex: 3, wantSource: "hop2", wantDestination: ""},
		{name: "beyond hop count", route: threeHopRoute, failureIndex: 4, wantSource: "", wantDestination: ""},
		{name: "max uint32", route: threeHopRoute, failureIndex: math.MaxUint32, wantSource: "", wantDestination: ""},
		{name: "nil source hop", route: routeWithNilSourceHop, failureIndex: 1, wantSource: "", wantDestination: "hop1"},
		{name: "nil destination hop", route: routeWithNilDestinationHop, failureIndex: 1, wantSource: "hop0", wantDestination: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, destination := routeFailureHops(tt.route, tt.failureIndex)
			if source != tt.wantSource {
				t.Errorf("source pubkey = %q, want %q", source, tt.wantSource)
			}
			if destination != tt.wantDestination {
				t.Errorf("destination pubkey = %q, want %q", destination, tt.wantDestination)
			}
		})
	}
}

// TestLNDMPPRouteIndexSafetyDoesNotPanicOrSpin drives PayInvoice over a bufconn
// backend for the malformed route shapes and failure indexes that previously
// panicked or spun forever. It asserts no panic, bounded termination, the
// existing no-route error, and that no invalid route is ever sent to LND.
func TestLNDMPPRouteIndexSafetyDoesNotPanicOrSpin(t *testing.T) {
	invoiceString, err := CreateMockInvoice(cashu.NewAmount(cashu.Sat, 100), "mpp route safety", chaincfg.MainNetParams, cashu.ExpiryTimeMinUnit(15))
	if err != nil {
		t.Fatalf("CreateMockInvoice: %v", err)
	}
	invoice, err := zpay32.Decode(invoiceString, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("zpay32.Decode: %v", err)
	}

	twoHopRoute := &lnrpc.Route{ //nolint:exhaustruct
		Hops: []*lnrpc.Hop{
			{PubKey: "hop0"}, //nolint:exhaustruct
			{PubKey: "hop1"}, //nolint:exhaustruct
		},
	}

	tests := []struct {
		name            string
		routes          []*lnrpc.Route
		responses       []*lnrpc.HTLCAttempt
		wantQueryCalls  int
		wantRouterCalls int
		wantErr         bool
	}{
		{
			name:            "empty route list",
			routes:          nil,
			responses:       nil,
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: 0,
		},
		{
			name:            "nil route",
			routes:          []*lnrpc.Route{nil},
			responses:       nil,
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: 0,
		},
		{
			name:            "empty hop list",
			routes:          []*lnrpc.Route{{}}, //nolint:exhaustruct
			responses:       nil,
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: 0,
		},
		{
			name:            "temporary failure at index zero",
			routes:          []*lnrpc.Route{twoHopRoute},
			responses:       []*lnrpc.HTLCAttempt{mppTemporaryChannelFailure(0)},
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: mppPartialPaymentAttempts,
		},
		{
			name:            "temporary failure at last hop index",
			routes:          []*lnrpc.Route{twoHopRoute},
			responses:       []*lnrpc.HTLCAttempt{mppTemporaryChannelFailure(2)},
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: mppPartialPaymentAttempts,
		},
		{
			name:            "temporary failure beyond hop count",
			routes:          []*lnrpc.Route{twoHopRoute},
			responses:       []*lnrpc.HTLCAttempt{mppTemporaryChannelFailure(99)},
			wantErr:         true,
			wantQueryCalls:  mppPartialPaymentAttempts,
			wantRouterCalls: mppPartialPaymentAttempts,
		},
		{
			name:            "temporary failure retried then settles",
			routes:          []*lnrpc.Route{twoHopRoute},
			responses:       []*lnrpc.HTLCAttempt{mppTemporaryChannelFailure(1), mppSucceededAttempt()},
			wantErr:         false,
			wantQueryCalls:  2,
			wantRouterCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lightningServer := &mppRouteSafetyLightningServer{routes: tt.routes} //nolint:exhaustruct
			routerServer := &mppRouteSafetyRouterServer{responses: tt.responses} //nolint:exhaustruct
			conn := startMPPRouteSafetyBackend(t, lightningServer, routerServer)

			response, err := mppPayInvoiceWithinDeadline(t, conn, invoiceString, invoice)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("PayInvoice error = nil, want error")
				}
				if !errors.Is(err, cashu.ErrPaymentNoRoute) {
					t.Errorf("PayInvoice error = %v, want wrapped %v", err, cashu.ErrPaymentNoRoute)
				}
			} else {
				if err != nil {
					t.Fatalf("PayInvoice error = %v, want nil", err)
				}
				if response.PaymentState != SETTLED {
					t.Errorf("PaymentState = %v, want %v", response.PaymentState, SETTLED)
				}
			}

			if got := lightningServer.callCount(); got != tt.wantQueryCalls {
				t.Errorf("QueryRoutes calls = %d, want %d", got, tt.wantQueryCalls)
			}
			if got := routerServer.callCount(); got != tt.wantRouterCalls {
				t.Errorf("SendToRouteV2 calls = %d, want %d", got, tt.wantRouterCalls)
			}
		})
	}
}

// mppInFlightAttempt is a SendToRouteV2 response for an HTLC that LND has
// accepted into the network but not yet resolved.
func mppInFlightAttempt() *lnrpc.HTLCAttempt {
	return &lnrpc.HTLCAttempt{Status: lnrpc.HTLCAttempt_IN_FLIGHT} //nolint:exhaustruct
}

// mppUnknownStatusAttempt is a SendToRouteV2 response whose status this build
// does not recognize. It must be treated as an ambiguous payment, never as a
// failure, and must not be retried.
func mppUnknownStatusAttempt() *lnrpc.HTLCAttempt {
	return &lnrpc.HTLCAttempt{Status: lnrpc.HTLCAttempt_HTLCStatus(99)} //nolint:exhaustruct
}

// mppTerminalFailureAttempt is a SendToRouteV2 response for a non-retryable
// failure (not TEMPORARY_CHANNEL_FAILURE).
func mppTerminalFailureAttempt() *lnrpc.HTLCAttempt {
	return &lnrpc.HTLCAttempt{ //nolint:exhaustruct
		Status: lnrpc.HTLCAttempt_FAILED,
		Failure: &lnrpc.Failure{ //nolint:exhaustruct
			Code: lnrpc.Failure_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS,
		},
	}
}

// TestLNDMPPPartialResponseStatusHandling pins the response-status handling of
// the unary SendToRouteV2 call in lndGrpcPayPartialInvoice. IN_FLIGHT must
// return promptly as PENDING without retrying (a retry could duplicate the
// payment), an unknown status must fail closed as an ambiguous PENDING without
// spinning or re-sending, and a non-retryable failure must report FAILED.
func TestLNDMPPPartialResponseStatusHandling(t *testing.T) {
	invoiceString, err := CreateMockInvoice(cashu.NewAmount(cashu.Sat, 100), "mpp response status", chaincfg.MainNetParams, cashu.ExpiryTimeMinUnit(15))
	if err != nil {
		t.Fatalf("CreateMockInvoice: %v", err)
	}
	invoice, err := zpay32.Decode(invoiceString, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("zpay32.Decode: %v", err)
	}

	twoHopRoute := &lnrpc.Route{ //nolint:exhaustruct
		Hops: []*lnrpc.Hop{
			{PubKey: "hop0"}, //nolint:exhaustruct
			{PubKey: "hop1"}, //nolint:exhaustruct
		},
	}

	tests := []struct {
		name            string
		wantErrContains string
		responses       []*lnrpc.HTLCAttempt
		wantState       PaymentStatus
		wantQueryCalls  int
		wantRouterCalls int
		wantErr         bool
		wantPaymentReq  bool
	}{
		{
			name:            "in flight returns pending without retry",
			wantErrContains: "",
			responses:       []*lnrpc.HTLCAttempt{mppInFlightAttempt()},
			wantState:       PENDING,
			wantQueryCalls:  1,
			wantRouterCalls: 1,
			wantErr:         false,
			wantPaymentReq:  true,
		},
		{
			name:            "unknown status fails closed without spin or resend",
			wantErrContains: "unknown HTLC attempt status",
			responses:       []*lnrpc.HTLCAttempt{mppUnknownStatusAttempt()},
			wantState:       PENDING,
			wantQueryCalls:  1,
			wantRouterCalls: 1,
			wantErr:         true,
			wantPaymentReq:  false,
		},
		{
			name:            "non-retryable failure reports failed",
			wantErrContains: "PaymentFailed",
			responses:       []*lnrpc.HTLCAttempt{mppTerminalFailureAttempt()},
			wantState:       FAILED,
			wantQueryCalls:  1,
			wantRouterCalls: 1,
			wantErr:         true,
			wantPaymentReq:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lightningServer := &mppRouteSafetyLightningServer{routes: []*lnrpc.Route{twoHopRoute}} //nolint:exhaustruct
			routerServer := &mppRouteSafetyRouterServer{responses: tt.responses}                   //nolint:exhaustruct
			conn := startMPPRouteSafetyBackend(t, lightningServer, routerServer)

			response, err := mppPayInvoiceWithinDeadline(t, conn, invoiceString, invoice)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("PayInvoice error = nil, want error")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("PayInvoice error = %q, want it to contain %q", err.Error(), tt.wantErrContains)
				}
				// A payment we could not classify must never be reported as
				// FAILED, otherwise the caller would free the proofs of a
				// possibly settled payment.
				if response.PaymentState == FAILED && tt.wantState != FAILED {
					t.Errorf("PaymentState = %v, want an ambiguous (non-FAILED) state", response.PaymentState)
				}
			} else if err != nil {
				t.Fatalf("PayInvoice error = %v, want nil", err)
			}

			if response.PaymentState != tt.wantState {
				t.Errorf("PaymentState = %v, want %v", response.PaymentState, tt.wantState)
			}
			if tt.wantPaymentReq && response.PaymentRequest != invoiceString {
				t.Errorf("PaymentRequest = %q, want %q", response.PaymentRequest, invoiceString)
			}
			if got := lightningServer.callCount(); got != tt.wantQueryCalls {
				t.Errorf("QueryRoutes calls = %d, want %d", got, tt.wantQueryCalls)
			}
			if got := routerServer.callCount(); got != tt.wantRouterCalls {
				t.Errorf("SendToRouteV2 calls = %d, want %d", got, tt.wantRouterCalls)
			}
		})
	}
}
