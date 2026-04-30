package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	ldkTestBitcoindRPCUser = "rpcuser"
	ldkTestBitcoindRPCPass = "rpcpassword"
	ldkTestBitcoindRPCPort = "18443"
	ldkTestBobLNDP2PPort   = "9736"
	ldkTestBitcoindWallet  = "wallet"
)

type LDKLightningNetworkEnv struct {
	Bitcoind    testcontainers.Container
	BobLnd      testcontainers.Container
	BitcoindRPC BitcoindRPCConfig
}

type BitcoindRPCConfig struct {
	Address  string
	Username string
	Password string
	Port     uint16
}

//nolint:govet
type bobChannelInfo struct {
	Active       bool   `json:"active"`
	LocalBalance string `json:"local_balance"`
}

type bobListChannelsResponse struct {
	Channels []bobChannelInfo `json:"channels"`
}

func SetupLDKLightningNetwork(t *testing.T, ctx context.Context, name string) (LDKLightningNetworkEnv, error) {
	t.Helper()
	cleanupCtx := context.WithoutCancel(ctx)

	netw, err := network.New(ctx, network.WithAttachable(), network.WithDriver("bridge"))
	if err != nil {
		return LDKLightningNetworkEnv{}, fmt.Errorf("network.New(...): %w", err)
	}
	t.Cleanup(func() {
		_ = netw.Remove(cleanupCtx)
	})

	bitcoind, err := setupLDKBitcoindContainer(ctx, netw.Name, name)
	if err != nil {
		return LDKLightningNetworkEnv{}, err
	}
	t.Cleanup(func() {
		_ = bitcoind.Terminate(cleanupCtx)
	})

	if err := createLDKBitcoindWallet(ctx, bitcoind); err != nil {
		return LDKLightningNetworkEnv{}, err
	}
	if err := generateLDKBitcoindBlocks(ctx, bitcoind, 101); err != nil {
		return LDKLightningNetworkEnv{}, err
	}

	bitcoindIP, err := bitcoind.ContainerIP(ctx)
	if err != nil {
		return LDKLightningNetworkEnv{}, fmt.Errorf("bitcoind.ContainerIP(...): %w", err)
	}
	bobLnd, err := setupLDKBobLndContainer(ctx, netw.Name, name, bitcoindIP)
	if err != nil {
		return LDKLightningNetworkEnv{}, err
	}
	t.Cleanup(func() {
		_ = bobLnd.Terminate(cleanupCtx)
	})

	if _, _, err := ldkBobEndpoint(ctx, bobLnd, 30*time.Second); err != nil {
		return LDKLightningNetworkEnv{}, err
	}

	return LDKLightningNetworkEnv{
		Bitcoind: bitcoind,
		BobLnd:   bobLnd,
		BitcoindRPC: BitcoindRPCConfig{
			Address:  bitcoindIP,
			Username: ldkTestBitcoindRPCUser,
			Password: ldkTestBitcoindRPCPass,
			Port:     18443,
		},
	}, nil
}

func (env LDKLightningNetworkEnv) FundAddress(ctx context.Context, address string, amountBTC string) error {
	if env.Bitcoind == nil {
		return fmt.Errorf("bitcoind container is nil")
	}
	err := execContainerCommandWithRetry(ctx, env.Bitcoind, []string{
		"bitcoin-cli",
		"-regtest",
		"-rpcuser=" + ldkTestBitcoindRPCUser,
		"-rpcpassword=" + ldkTestBitcoindRPCPass,
		"-rpcwallet=" + ldkTestBitcoindWallet,
		"sendtoaddress",
		address,
		amountBTC,
	})
	if err != nil {
		return fmt.Errorf("bitcoind sendtoaddress: %w", err)
	}
	return nil
}

func (env LDKLightningNetworkEnv) MineBlocks(ctx context.Context, count int) error {
	if env.Bitcoind == nil {
		return fmt.Errorf("bitcoind container is nil")
	}
	return generateLDKBitcoindBlocks(ctx, env.Bitcoind, count)
}

func (env LDKLightningNetworkEnv) BobEndpoint(ctx context.Context) (string, string, error) {
	if env.BobLnd == nil {
		return "", "", fmt.Errorf("bob lnd container is nil")
	}
	return ldkBobEndpoint(ctx, env.BobLnd, 30*time.Second)
}

func (env LDKLightningNetworkEnv) WaitForBobOutbound(ctx context.Context, minLocalBalanceSat uint64, timeout time.Duration) error {
	if env.BobLnd == nil {
		return fmt.Errorf("bob lnd container is nil")
	}
	deadline := time.Now().Add(timeout)
	var lastResp bobListChannelsResponse
	for time.Now().Before(deadline) {
		output, err := execContainerCommand(ctx, env.BobLnd, []string{
			"lncli",
			"--tlscertpath", "/home/lnd/.lnd/tls.cert",
			"--macaroonpath", "/home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon",
			"listchannels",
		})
		if err == nil {
			jsonStart := strings.IndexByte(output, '{')
			if jsonStart != -1 {
				if err := json.Unmarshal([]byte(output[jsonStart:]), &lastResp); err == nil {
					for _, channel := range lastResp.Channels {
						localBalance, parseErr := strconv.ParseUint(channel.LocalBalance, 10, 64)
						if parseErr == nil && channel.Active && localBalance >= minLocalBalanceSat {
							return nil
						}
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for bob outbound balance >= %d, last_channels=%+v", minLocalBalanceSat, lastResp.Channels)
}

func setupLDKBitcoindContainer(ctx context.Context, networkName string, name string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{ //nolint:exhaustruct
		Image:        "polarlightning/bitcoind:29.0",
		Name:         "ldkBitcoind" + name,
		WaitingFor:   wait.ForLog("Initialized HTTP server"),
		ExposedPorts: []string{"18443/tcp", "18444/tcp", "28334/tcp", "28335/tcp", "28336/tcp"},
		Networks:     []string{networkName},
		Cmd: []string{
			"bitcoind",
			"-server=1",
			"-regtest=1",
			"-rpcuser=" + ldkTestBitcoindRPCUser,
			"-rpcpassword=" + ldkTestBitcoindRPCPass,
			"-debug=1",
			"-zmqpubrawblock=tcp://0.0.0.0:28334",
			"-zmqpubrawtx=tcp://0.0.0.0:28335",
			"-zmqpubhashblock=tcp://0.0.0.0:28336",
			"-txindex=1",
			"-dnsseed=0",
			"-upnp=0",
			"-rpcbind=0.0.0.0",
			"-rpcallowip=0.0.0.0/0",
			"-rpcport=" + ldkTestBitcoindRPCPort,
			"-rest",
			"-listen=1",
			"-listenonion=0",
			"-fallbackfee=0.0002",
			"-blockfilterindex=1",
			"-peerblockfilters=1",
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ //nolint:exhaustruct
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testcontainers.GenericContainer(bitcoind): %w", err)
	}
	return container, nil
}

func createLDKBitcoindWallet(ctx context.Context, bitcoind testcontainers.Container) error {
	err := execContainerCommandWithRetry(ctx, bitcoind, []string{
		"bitcoin-cli",
		"-regtest",
		"-rpcuser=" + ldkTestBitcoindRPCUser,
		"-rpcpassword=" + ldkTestBitcoindRPCPass,
		"-named",
		"createwallet",
		"wallet_name=" + ldkTestBitcoindWallet,
	})
	if err != nil {
		return fmt.Errorf("bitcoind createwallet: %w", err)
	}
	return nil
}

func generateLDKBitcoindBlocks(ctx context.Context, bitcoind testcontainers.Container, count int) error {
	err := execContainerCommandWithRetry(ctx, bitcoind, []string{
		"bitcoin-cli",
		"-regtest",
		"-rpcuser=" + ldkTestBitcoindRPCUser,
		"-rpcpassword=" + ldkTestBitcoindRPCPass,
		"-rpcwallet=" + ldkTestBitcoindWallet,
		"-generate",
		strconv.Itoa(count),
	})
	if err != nil {
		return fmt.Errorf("bitcoind generate %d blocks: %w", count, err)
	}
	return nil
}

func setupLDKBobLndContainer(ctx context.Context, networkName string, name string, bitcoindIP string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{ //nolint:exhaustruct
		Image:        "polarlightning/lnd:0.19.2-beta",
		Name:         "ldkBob" + name,
		WaitingFor:   wait.ForLog("RPC server listening on").AsRegexp(),
		ExposedPorts: []string{"9736/tcp", "10009/tcp", "8081/tcp"},
		Networks:     []string{networkName},
		Cmd: []string{
			"lnd",
			"--noseedbackup",
			"--trickledelay=5000",
			"--alias=bob",
			"--tlsextradomain=bob",
			"--tlsextradomain=host.docker.bridge",
			"--tlsextradomain=host.docker.internal",
			"--listen=0.0.0.0:" + ldkTestBobLNDP2PPort,
			"--rpclisten=0.0.0.0:10009",
			"--restlisten=0.0.0.0:8081",
			"--bitcoin.active",
			"--bitcoin.regtest",
			"--bitcoin.node=bitcoind",
			"--bitcoind.rpchost=" + bitcoindIP,
			"--bitcoind.rpcuser=" + ldkTestBitcoindRPCUser,
			"--bitcoind.rpcpass=" + ldkTestBitcoindRPCPass,
			"--bitcoind.zmqpubrawblock=tcp://" + bitcoindIP + ":28334",
			"--bitcoind.zmqpubrawtx=tcp://" + bitcoindIP + ":28335",
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ //nolint:exhaustruct
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testcontainers.GenericContainer(bob lnd): %w", err)
	}
	return container, nil
}

type ldkLndInfo struct {
	IdentityPubkey string   `json:"identity_pubkey"`
	URIs           []string `json:"uris"`
}

func ldkBobEndpoint(ctx context.Context, bobLnd testcontainers.Container, timeout time.Duration) (string, string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		info, err := ldkBobInfo(ctx, bobLnd)
		if err == nil && strings.TrimSpace(info.IdentityPubkey) != "" {
			address, addressErr := selectReachableBobAddress(ctx, bobLnd, info.URIs)
			if addressErr == nil {
				return info.IdentityPubkey, address, nil
			}
			lastErr = addressErr
		} else if err != nil {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for bob endpoint")
	}
	return "", "", lastErr
}

func ldkBobInfo(ctx context.Context, bobLnd testcontainers.Container) (ldkLndInfo, error) {
	output, err := execContainerCommand(ctx, bobLnd, []string{
		"lncli",
		"--tlscertpath", "/home/lnd/.lnd/tls.cert",
		"--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon",
		"getinfo",
	})
	if err != nil {
		return ldkLndInfo{}, fmt.Errorf("bob lncli getinfo: %w", err)
	}
	data := []byte(output)
	jsonStart := strings.IndexByte(output, '{')
	if jsonStart == -1 {
		return ldkLndInfo{}, fmt.Errorf("getinfo output did not contain json")
	}

	var info ldkLndInfo
	if err := json.Unmarshal(data[jsonStart:], &info); err != nil {
		return ldkLndInfo{}, fmt.Errorf("json.Unmarshal(getinfo): %w", err)
	}
	return info, nil
}

func selectReachableBobAddress(ctx context.Context, bobLnd testcontainers.Container, uris []string) (string, error) {
	for _, uri := range uris {
		_, hostPort, found := strings.Cut(uri, "@")
		if !found {
			continue
		}
		host, _, err := net.SplitHostPort(hostPort)
		if err != nil {
			continue
		}
		if net.ParseIP(host) == nil || strings.Contains(host, ":") {
			continue
		}
		if reachableBridgeAddress(hostPort) {
			return hostPort, nil
		}
	}

	host, err := bobLnd.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("bobLnd.Host(...): %w", err)
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	mappedPort, err := bobLnd.MappedPort(ctx, ldkTestBobLNDP2PPort+"/tcp")
	if err != nil {
		return "", fmt.Errorf("bobLnd.MappedPort(...): %w", err)
	}
	endpoint := net.JoinHostPort(host, mappedPort.Port())
	if !reachableBridgeAddress(endpoint) {
		return "", fmt.Errorf("bob endpoint %q is not reachable", endpoint)
	}
	return endpoint, nil
}

func reachableBridgeAddress(endpoint string) bool {
	conn, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func execContainerCommand(ctx context.Context, container testcontainers.Container, cmd []string) (string, error) {
	exitCode, reader, err := container.Exec(ctx, cmd)
	if err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(reader)
	if readErr != nil {
		return "", readErr
	}
	output := strings.TrimSpace(string(data))
	if exitCode != 0 {
		return output, fmt.Errorf("exit code %d: %s", exitCode, output)
	}
	return output, nil
}

func execContainerCommandWithRetry(ctx context.Context, container testcontainers.Container, cmd []string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		_, err := execContainerCommand(ctx, container, cmd)
		if err == nil {
			return nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "error code: -28") && !strings.Contains(err.Error(), "Verifying blocks") {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}
