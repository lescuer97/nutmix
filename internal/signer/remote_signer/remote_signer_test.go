package remotesigner

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
	"github.com/lescuer97/nutmix/internal/signer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type blockingSignatory struct {
	sig.UnimplementedSignatoryServer
	requestCtx chan context.Context
}

func (s *blockingSignatory) BlindSign(ctx context.Context, _ *sig.BlindedMessages) (*sig.BlindSignResponse, error) {
	s.requestCtx <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRejectsNilBlindedMessage(t *testing.T) {
	messages := []cashu.BlindedMessage{{B_: cashu.WrappedPublicKey{PublicKey: nil}, Id: "", Witness: "", Amount: 0}}

	_, _, err := (&RemoteSigner{grpcClient: nil, activeKeysets: nil, keysets: nil, pubkey: nil}).SignBlindMessages(context.Background(), messages)
	if !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Errorf("SignBlindMessages error = %v, want ErrInvalidBlindMessage", err)
	}

	_, err = ConvertBlindedMessagedToGRPC(messages)
	if !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Errorf("ConvertBlindedMessagedToGRPC error = %v, want ErrInvalidBlindMessage", err)
	}
}

func TestSignBlindMessagesHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := (&RemoteSigner{grpcClient: nil, activeKeysets: nil, keysets: nil, pubkey: nil}).SignBlindMessages(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SignBlindMessages error = %v, want context.Canceled", err)
	}
}

func TestSignBlindMessagesPassesParentDeadlineToRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	service := &blockingSignatory{UnimplementedSignatoryServer: sig.UnimplementedSignatoryServer{}, requestCtx: make(chan context.Context, 1)}
	sig.RegisterSignatoryServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(clientVersionInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	parentDeadline := time.Now().Add(2 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), parentDeadline)
	defer cancel()
	remote := &RemoteSigner{grpcClient: sig.NewSignatoryClient(conn), activeKeysets: nil, keysets: nil, pubkey: nil}
	result := make(chan error, 1)
	go func() {
		_, _, err := remote.SignBlindMessages(ctx, nil)
		result <- err
	}()

	select {
	case rpcCtx := <-service.requestCtx:
		rpcDeadline, ok := rpcCtx.Deadline()
		if !ok || rpcDeadline.Before(parentDeadline.Add(-20*time.Millisecond)) || rpcDeadline.After(parentDeadline.Add(20*time.Millisecond)) {
			t.Fatalf("RPC deadline = %v, %v; want parent deadline %v", rpcDeadline, ok, parentDeadline)
		}
		if md, ok := metadata.FromIncomingContext(rpcCtx); !ok || len(md.Get("x-signatory-schema-version")) != 1 {
			t.Fatalf("RPC metadata = %v, %v; want signatory schema version", md, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("BlindSign RPC did not receive request")
	}
	cancel()
	select {
	case err := <-result:
		if status.Code(err) != codes.Canceled {
			t.Fatalf("SignBlindMessages() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SignBlindMessages did not stop after parent cancellation")
	}
}

func TestSignBlindMessagesCapsRPCDeadline(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	service := &blockingSignatory{UnimplementedSignatoryServer: sig.UnimplementedSignatoryServer{}, requestCtx: make(chan context.Context, 1)}
	sig.RegisterSignatoryServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(clientVersionInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	remote := &RemoteSigner{grpcClient: sig.NewSignatoryClient(conn), activeKeysets: nil, keysets: nil, pubkey: nil}
	result := make(chan error, 1)
	go func() {
		_, _, err := remote.SignBlindMessages(ctx, nil)
		result <- err
	}()

	select {
	case rpcCtx := <-service.requestCtx:
		rpcDeadline, ok := rpcCtx.Deadline()
		remaining := time.Until(rpcDeadline)
		if !ok || remaining > signer.SignBlindMessagesTimeout || remaining < signer.SignBlindMessagesTimeout-time.Second {
			t.Fatalf("RPC deadline remaining = %v, present = %v; want at most %v", remaining, ok, signer.SignBlindMessagesTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("BlindSign RPC did not receive request")
	}
	cancel()
	select {
	case err := <-result:
		if status.Code(err) != codes.Canceled {
			t.Fatalf("SignBlindMessages() error = %v, want canceled RPC", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SignBlindMessages did not stop after parent cancellation")
	}
}
