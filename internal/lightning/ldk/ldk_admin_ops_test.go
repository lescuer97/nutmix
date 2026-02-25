package ldk

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestNewOnchainAddressNilReceiver(t *testing.T) {
	var backend *LDK

	_, err := backend.NewOnchainAddress()
	if err == nil {
		t.Fatal("expected error for nil ldk backend")
	}
}

func TestNewOnchainAddressUninitializedNode(t *testing.T) {
	backend := &LDK{}

	_, err := backend.NewOnchainAddress()
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
}

func TestSendOnchainNilReceiver(t *testing.T) {
	var backend *LDK

	err := backend.SendOnchain("bcrt1qexample", 1000)
	if err == nil {
		t.Fatal("expected error for nil ldk backend")
	}
}

func TestSendOnchainUninitializedNode(t *testing.T) {
	backend := &LDK{}

	err := backend.SendOnchain("bcrt1qexample", 1000)
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
}

func TestSendOnchainPreservesGetNodeError(t *testing.T) {
	backend := &LDK{}

	err := backend.SendOnchain("bcrt1qexample", 1000)
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
	if !strings.Contains(err.Error(), "ldk node is not initialized") {
		t.Fatalf("expected wrapped getNode error, got %v", err)
	}
}

func TestValidateOnchainSendAddressRejectsMalformed(t *testing.T) {
	err := validateOnchainSendAddress("not-a-bitcoin-address", &chaincfg.MainNetParams)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "invalid") {
		t.Fatalf("expected invalid address error, got %v", err)
	}
	if !IsOnchainSendValidationError(err) {
		t.Fatalf("expected validation error type, got %T", err)
	}
}

func TestValidateOnchainSendAddressRejectsWrongNetwork(t *testing.T) {
	err := validateOnchainSendAddress("1BoatSLRHtKNngkdXEeobR76b53LETtpyT", &chaincfg.TestNet3Params)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "network") {
		t.Fatalf("expected wrong-network error, got %v", err)
	}
	if !IsOnchainSendValidationError(err) {
		t.Fatalf("expected validation error type, got %T", err)
	}
}

func TestValidateOnchainSendAmount(t *testing.T) {
	if err := validateOnchainSendAmount(1000, 1000); err != nil {
		t.Fatalf("validateOnchainSendAmount(max,max) returned error: %v", err)
	}
	if err := validateOnchainSendAmount(0, 1000); err == nil {
		t.Fatal("expected zero amount error")
	} else if !IsOnchainSendValidationError(err) {
		t.Fatalf("expected validation error type, got %T", err)
	}
	if err := validateOnchainSendAmount(1000, 999); err == nil {
		t.Fatal("expected overspend error")
	} else if !IsOnchainSendValidationError(err) {
		t.Fatalf("expected validation error type, got %T", err)
	}
}

func TestOpenChannelNilReceiver(t *testing.T) {
	var backend *LDK

	err := backend.OpenChannel("02abc", "127.0.0.1:9735", 1000)
	if err == nil {
		t.Fatal("expected error for nil ldk backend")
	}
}

func TestOpenChannelUninitializedNode(t *testing.T) {
	backend := &LDK{}

	err := backend.OpenChannel("02abc", "127.0.0.1:9735", 1000)
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
}

func TestCloseChannelNilReceiver(t *testing.T) {
	var backend *LDK

	err := backend.CloseChannel("chan-1", "02abc")
	if err == nil {
		t.Fatal("expected error for nil ldk backend")
	}
}

func TestCloseChannelUninitializedNode(t *testing.T) {
	backend := &LDK{}

	err := backend.CloseChannel("chan-1", "02abc")
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
}

func TestForceCloseChannelNilReceiver(t *testing.T) {
	var backend *LDK

	err := backend.ForceCloseChannel("chan-1", "02abc")
	if err == nil {
		t.Fatal("expected error for nil ldk backend")
	}
}

func TestForceCloseChannelUninitializedNode(t *testing.T) {
	backend := &LDK{}

	err := backend.ForceCloseChannel("chan-1", "02abc")
	if err == nil {
		t.Fatal("expected error for uninitialized ldk node")
	}
}
