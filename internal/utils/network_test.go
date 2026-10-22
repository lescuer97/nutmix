package utils

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestCheckChainParams(t *testing.T) {
	tests := []struct {
		name    string
		network string
		wantNet uint32
		wantErr bool
	}{
		{name: "testnet3", network: "testnet3", wantNet: uint32(chaincfg.TestNet3Params.Net), wantErr: false},
		{name: "testnet alias", network: "testnet", wantNet: uint32(chaincfg.TestNet3Params.Net), wantErr: false},
		{name: "mainnet", network: "mainnet", wantNet: uint32(chaincfg.MainNetParams.Net), wantErr: false},
		{name: "regtest", network: "regtest", wantNet: uint32(chaincfg.RegressionNetParams.Net), wantErr: false},
		{name: "signet", network: "signet", wantNet: uint32(chaincfg.SigNetParams.Net), wantErr: false},
		{name: "invalid", network: "nope", wantNet: uint32(chaincfg.MainNetParams.Net), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := CheckChainParams(tt.network)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for network %q", tt.network)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CheckChainParams(%q): %v", tt.network, err)
			}

			if uint32(params.Net) != tt.wantNet {
				t.Fatalf("params.Net mismatch, got %v, want %v", params.Net, tt.wantNet)
			}
		})
	}
}
