package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
)

func TestShutdownServerAndBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := shutdownServerAndBackend(ctx, server.Config, nil); err != nil {
		t.Fatalf("shutdownServerAndBackend() error = %v", err)
	}
}

func TestStopLDKBackendUsesDirectBackend(t *testing.T) {
	backend, err := ldk.NewConfigBackend(nil, ldk.LdkConfig{})
	if err != nil {
		t.Fatal(err)
	}
	err = stopLDKBackend(t.Context(), &mint.Mint{LightningBackend: backend}) //nolint:exhaustruct
	if err != nil {
		t.Fatalf("stopLDKBackend() error = %v", err)
	}
}
