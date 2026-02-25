package utils

import (
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
)

func CheckChainParams(network string) (chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet3", "testnet":
		return chaincfg.TestNet3Params, nil
	case "mainnet":
		return chaincfg.MainNetParams, nil
	case "regtest":
		return chaincfg.RegressionNetParams, nil
	case "signet":
		return chaincfg.SigNetParams, nil
	default:
		return chaincfg.MainNetParams, fmt.Errorf("invalid network: %s", network)
	}
}
