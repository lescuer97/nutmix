package lightning

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cln_grpc "github.com/lescuer97/nutmix/internal/lightning/proto"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestFakeWalletStatus(t *testing.T) {
	tests := []struct {
		name   string
		status NodeStatus
		want   NodeStatus
	}{
		{name: "defaults online", status: "", want: ONLINE_STATUS},
		{name: "offline override", status: OFFLINE_STATUS, want: OFFLINE_STATUS},
		{name: "unknown override", status: UNKNOWN_STATUS, want: UNKNOWN_STATUS},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (FakeWallet{NodeStatus: test.status}).Status(context.Background()) //nolint:exhaustruct
			if err != nil {
				t.Fatalf("Status() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Status() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLnbitsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/wallet" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Api-Key") != "valid" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"wallet","balance":0}`))
	}))
	t.Cleanup(server.Close)

	assertStatus(t, LnbitsWallet{Endpoint: server.URL, Key: "valid"}.Status, ONLINE_STATUS, false)   //nolint:exhaustruct
	assertStatus(t, LnbitsWallet{Endpoint: server.URL, Key: "invalid"}.Status, OFFLINE_STATUS, true) //nolint:exhaustruct
}

func TestLnbitsStatusCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	assertStatusCancellation(t, started, LnbitsWallet{Endpoint: server.URL, Key: "valid"}.Status) //nolint:exhaustruct
}

type lndStatusServer struct {
	lnrpc.UnimplementedLightningServer
	err     error
	started chan<- struct{}
	release <-chan struct{}
}

func (s lndStatusServer) GetInfo(ctx context.Context, _ *lnrpc.GetInfoRequest) (*lnrpc.GetInfoResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("macaroon")) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing macaroon")
	}
	if s.started != nil {
		close(s.started)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.release:
		}
	}
	return &lnrpc.GetInfoResponse{}, s.err
}

func TestLndStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		want    NodeStatus
		wantErr bool
	}{
		{name: "online", err: nil, want: ONLINE_STATUS, wantErr: false},
		{name: "offline", err: status.Error(codes.Unavailable, "offline"), want: OFFLINE_STATUS, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn := newStatusTestConn(t, func(server *grpc.Server) {
				lnrpc.RegisterLightningServer(server, lndStatusServer{UnimplementedLightningServer: lnrpc.UnimplementedLightningServer{}, err: test.err, started: nil, release: nil})
			})
			assertStatus(t, (LndGrpcWallet{grpcClient: conn, macaroon: "test"}).Status, test.want, test.wantErr) //nolint:exhaustruct
		})
	}
}

func TestLndStatusCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	conn := newStatusTestConn(t, func(server *grpc.Server) {
		lnrpc.RegisterLightningServer(server, lndStatusServer{
			UnimplementedLightningServer: lnrpc.UnimplementedLightningServer{},
			started:                      started,
			release:                      release,
			err:                          nil,
		})
	})
	t.Cleanup(func() { close(release) })

	assertStatusCancellation(t, started, (LndGrpcWallet{grpcClient: conn, macaroon: "test"}).Status) //nolint:exhaustruct
}

type clnStatusServer struct {
	cln_grpc.UnimplementedNodeServer
	err     error
	started chan<- struct{}
	release <-chan struct{}
}

func (s clnStatusServer) Getinfo(ctx context.Context, _ *cln_grpc.GetinfoRequest) (*cln_grpc.GetinfoResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("rune")) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing rune")
	}
	if s.started != nil {
		close(s.started)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.release:
		}
	}
	return &cln_grpc.GetinfoResponse{}, s.err
}

func TestCLNStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		want    NodeStatus
		wantErr bool
	}{
		{name: "online", err: nil, want: ONLINE_STATUS, wantErr: false},
		{name: "offline", err: status.Error(codes.Unavailable, "offline"), want: OFFLINE_STATUS, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn := newStatusTestConn(t, func(server *grpc.Server) {
				cln_grpc.RegisterNodeServer(server, clnStatusServer{UnimplementedNodeServer: cln_grpc.UnimplementedNodeServer{}, err: test.err, started: nil, release: nil})
			})
			assertStatus(t, (CLNGRPCWallet{grpcClient: conn, macaroon: "test"}).Status, test.want, test.wantErr) //nolint:exhaustruct
		})
	}
}

func TestCLNStatusCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	conn := newStatusTestConn(t, func(server *grpc.Server) {
		cln_grpc.RegisterNodeServer(server, clnStatusServer{
			UnimplementedNodeServer: cln_grpc.UnimplementedNodeServer{},
			started:                 started,
			release:                 release,
			err:                     nil,
		})
	})
	t.Cleanup(func() { close(release) })

	assertStatusCancellation(t, started, (CLNGRPCWallet{grpcClient: conn, macaroon: "test"}).Status) //nolint:exhaustruct
}

func assertStatus(t *testing.T, check func(context.Context) (NodeStatus, error), want NodeStatus, wantErr bool) {
	t.Helper()
	got, err := check(context.Background())
	if (err != nil) != wantErr {
		t.Fatalf("Status() error = %v, wantErr %v", err, wantErr)
	}
	if got != want {
		t.Fatalf("Status() = %q, want %q", got, want)
	}
}

func assertStatusCancellation(t *testing.T, started <-chan struct{}, check func(context.Context) (NodeStatus, error)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		err    error
		status NodeStatus
	}
	resultChan := make(chan result, 1)
	go func() {
		got, err := check(ctx)
		resultChan <- result{status: got, err: err}
	}()

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("status request did not reach server within 500ms")
	}
	cancel()

	select {
	case result := <-resultChan:
		if result.status != OFFLINE_STATUS {
			t.Fatalf("Status() = %q, want %q", result.status, OFFLINE_STATUS)
		}
		if result.err == nil {
			t.Fatal("Status() error = nil, want non-nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Status() did not return within 500ms after cancellation")
	}
}

func newStatusTestConn(t *testing.T, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	register(server)
	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	})
	return conn
}
