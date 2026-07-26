package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
