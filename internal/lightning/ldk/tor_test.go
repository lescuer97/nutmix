package ldk

import (
	"crypto/sha3"
	"encoding/base32"
	"strings"
	"testing"
)

func validTorV3Address(t *testing.T) string {
	t.Helper()

	publicKey := make([]byte, 32, 35)
	for i := range publicKey {
		publicKey[i] = byte(i)
	}
	checksumInput := append([]byte(".onion checksum"), publicKey...)
	checksumInput = append(checksumInput, 3)
	checksum := sha3.Sum256(checksumInput)
	payload := append(publicKey, checksum[:2]...)
	payload = append(payload, 3)

	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(payload)) + ".onion:9735"
}

func TestValidateTorV3SocketAddress(t *testing.T) {
	validAddress := validTorV3Address(t)

	for _, address := range []string{
		validAddress,
		strings.ToUpper(strings.TrimSuffix(validAddress, ":9735")) + ":9735",
	} {
		if err := validateTorV3SocketAddress(address); err != nil {
			t.Fatalf("validateTorV3SocketAddress(%q): %v", address, err)
		}
	}

	for _, address := range []string{
		"abcdefghijklmnop.onion:9735",
		"example.com:9735",
		"127.0.0.1:9735",
		strings.TrimSuffix(validAddress, ":9735"),
		strings.TrimSuffix(validAddress, "9735") + "0",
		"invalid.onion:9735",
	} {
		if err := validateTorV3SocketAddress(address); err == nil {
			t.Fatalf("expected %q to be rejected", address)
		}
	}
}

func TestValidateTorOnlyListeningAddresses(t *testing.T) {
	if err := validateTorOnlyListeningAddresses(nil); err != nil {
		t.Fatalf("validateTorOnlyListeningAddresses(nil): %v", err)
	}
	for _, address := range []string{validTorV3Address(t), "127.0.0.1:9735", "localhost:9735", "[::1]:9735"} {
		if err := validateTorOnlyListeningAddresses([]string{address}); err != nil {
			t.Fatalf("validateTorOnlyListeningAddresses(%q): %v", address, err)
		}
	}
	if err := validateTorOnlyListeningAddresses([]string{"8.8.8.8:9735"}); err == nil {
		t.Fatal("expected clearnet listening address to be rejected")
	}
}

func TestValidateTorProxySocketAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:9050", "localhost:9150", "[::1]:9050"} {
		if err := validateTorProxySocketAddress(address); err != nil {
			t.Fatalf("validateTorProxySocketAddress(%q): %v", address, err)
		}
	}

	for _, address := range []string{"", ":9050", "localhost", "localhost:0", "localhost:65536"} {
		if err := validateTorProxySocketAddress(address); err == nil {
			t.Fatalf("expected %q to be rejected", address)
		}
	}
}

func TestValidatePersistedConfigTorOnlyWithoutProxy(t *testing.T) {
	config := mustPersistedConfig(t, t.TempDir())
	config.TorOnly = true
	if err := validatePersistedConfig(config); err != nil {
		t.Fatalf("validatePersistedConfig(config): %v", err)
	}

	proxyAddress := "127.0.0.1:9050"
	config.TorProxyAddress = &proxyAddress
	if err := validatePersistedConfig(config); err != nil {
		t.Fatalf("validatePersistedConfig(config): %v", err)
	}
}

func TestValidatePersistedConfigTorOnlyChainSourceHosts(t *testing.T) {
	onionHost := strings.TrimSuffix(validTorV3Address(t), ":9735")

	bitcoindConfig := func(address string) PersistedConfig {
		config := mustPersistedConfig(t, t.TempDir())
		config.TorOnly = true
		config.Rpc.Address = address
		return config
	}

	for _, address := range []string{"127.0.0.1", "localhost", "::1", onionHost} {
		if err := validatePersistedConfig(bitcoindConfig(address)); err != nil {
			t.Fatalf("expected bitcoind address %q to be accepted: %v", address, err)
		}
	}
	for _, address := range []string{"8.8.8.8", "example.com", "abcdefghijklmnop.onion"} {
		if err := validatePersistedConfig(bitcoindConfig(address)); err == nil {
			t.Fatalf("expected bitcoind address %q to be rejected", address)
		}
	}

	electrumConfig := func(serverURL string) PersistedConfig {
		config := mustPersistedConfig(t, t.TempDir())
		config.TorOnly = true
		config.ChainSourceType = ChainSourceElectrum
		config.ElectrumServerURL = serverURL
		return config
	}

	for _, serverURL := range []string{"ssl://127.0.0.1:50002", "tcp://localhost:50001", "ssl://" + onionHost + ":50002"} {
		if err := validatePersistedConfig(electrumConfig(serverURL)); err != nil {
			t.Fatalf("expected electrum url %q to be accepted: %v", serverURL, err)
		}
	}
	if err := validatePersistedConfig(electrumConfig("ssl://electrum.example.com:50002")); err == nil {
		t.Fatal("expected clearnet electrum url to be rejected")
	}

	esploraConfig := func(serverURL string) PersistedConfig {
		config := mustPersistedConfig(t, t.TempDir())
		config.TorOnly = true
		config.ChainSourceType = ChainSourceEsplora
		config.EsploraServerURL = serverURL
		return config
	}

	for _, serverURL := range []string{"http://127.0.0.1:3002", "https://" + onionHost} {
		if err := validatePersistedConfig(esploraConfig(serverURL)); err != nil {
			t.Fatalf("expected esplora url %q to be accepted: %v", serverURL, err)
		}
	}
	if err := validatePersistedConfig(esploraConfig("https://mempool.space")); err == nil {
		t.Fatal("expected clearnet esplora url to be rejected")
	}

	// clearnet chain sources stay valid when tor only is disabled
	config := electrumConfig("ssl://electrum.example.com:50002")
	config.TorOnly = false
	if err := validatePersistedConfig(config); err != nil {
		t.Fatalf("expected clearnet electrum url to be accepted without tor only: %v", err)
	}
}

func TestOpenChannelRejectsClearnetWhenTorOnly(t *testing.T) {
	err := (&LDK{config: LdkConfig{TorOnly: true}}).OpenChannel("02abc", "8.8.8.8:9735", 1000)
	if err == nil || !strings.Contains(err.Error(), "tor-only") {
		t.Fatalf("expected tor-only validation error, got %v", err)
	}
}
