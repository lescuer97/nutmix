package main

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	pq "github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMintBolt11LDKLightning(t *testing.T) {
	const postgresPassword = "password"
	const postgresUser = "user"

	ctx := t.Context()
	tempDir := t.TempDir()

	postgresContainer, err := postgres.Run(ctx, "postgres:16.2",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = postgresContainer.Terminate(context.Background())
	})

	connURI, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("postgresContainer.ConnectionString(...): %v", err)
	}
	t.Setenv("DATABASE_URL", connURI)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv(mint.NETWORK_ENV, "regtest")

	env, err := utils.SetupLDKLightningNetwork(t, ctx, "ldk-bolt11-tests")
	if err != nil {
		t.Fatalf("utils.SetupLDKLightningNetwork(...): %v", err)
	}

	db, err := pq.DatabaseSetup(ctx, "../../migrations/")
	if err != nil {
		t.Fatalf("pq.DatabaseSetup(...): %v", err)
	}
	ldkConfig, err := ldk.NewPersistedConfig(ldk.RPCConfig{
		Address:  env.BitcoindRPC.Address,
		Port:     env.BitcoindRPC.Port,
		Username: env.BitcoindRPC.Username,
		Password: env.BitcoindRPC.Password,
	}, tempDir)
	if err != nil {
		t.Fatalf("ldk.NewPersistedConfig(...): %v", err)
	}
	if err := ldk.SaveConfig(ctx, db, ldkConfig); err != nil {
		t.Fatalf("ldk.SaveConfig(...): %v", err)
	}

	setupBackend, err := ldk.NewLdk(ctx, db, "regtest")
	if err != nil {
		t.Fatalf("ldk.NewLdk(...): %v", err)
	}
	t.Cleanup(func() {
		_ = setupBackend.Stop()
	})
	if err := waitForBestBlock(t, setupBackend, 101, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	address, err := setupBackend.NewOnchainAddress()
	if err != nil {
		t.Fatalf("setupBackend.NewOnchainAddress(): %v", err)
	}
	if err := env.FundAddress(ctx, address, "10"); err != nil {
		t.Fatalf("env.FundAddress(...): %v", err)
	}
	if err := env.MineBlocks(ctx, 10); err != nil {
		t.Fatalf("env.MineBlocks(10): %v", err)
	}
	if err := waitForOnchainBalance(t, setupBackend, 90*time.Second); err != nil {
		t.Fatal(err)
	}

	pubkey, endpoint, err := env.BobEndpoint(ctx)
	if err != nil {
		t.Fatalf("env.BobEndpoint(...): %v", err)
	}
	if err := openChannelWithRetry(t, setupBackend, pubkey, endpoint, 1_000_000, 150_000*1000, 90*time.Second); err != nil {
		t.Fatalf("setupBackend.OpenChannel(...): %v", err)
	}
	if err := waitForChannelState(t, setupBackend, pubkey, 60*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := env.MineBlocks(ctx, 10); err != nil {
		t.Fatalf("env.MineBlocks(10): %v", err)
	}
	if err := waitForBootstrapReady(t, setupBackend, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := env.WaitForBobOutbound(ctx, 1_000, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.LDK))
	err = setupBackend.Stop()
	if err != nil {
		t.Fatalf("could not stop the setup ln node. %+v", err)
	}
	runtime.GC()
	runtime.GC()

	router, mint := SetupRoutingForTesting(ctx, false)
	if currentLDKBackend, ok := mint.LightningBackend.(*ldk.LDK); ok {
		if err := currentLDKBackend.Stop(); err != nil {
			t.Fatalf("could not stop the setup routing ldk node. %+v", err)
		}
	}
	mint.LightningBackend = nil
	runtime.GC()
	runtime.GC()

	mintBackend, err := ldk.NewLdk(ctx, db, "regtest")
	if err != nil {
		t.Fatalf("ldk.NewLdk(...): %v", err)
	}
	mint.LightningBackend = mintBackend
	t.Cleanup(func() {
		_ = mintBackend.Stop()
	})
	if err := waitForLDKMintReady(t, mintBackend, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	LightningBolt11Test(t, ctx, router, mint, env.BobLnd)
}

func waitForOnchainBalance(t *testing.T, backend *ldk.LDK, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastBalances ldk.LDKBalances
	var lastErr error
	for time.Now().Before(deadline) {
		if err := backend.SyncWallets(); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		balances, err := backend.Balances()
		if err == nil {
			lastBalances = balances
		} else {
			lastErr = err
		}
		if err == nil && balances.AvailableOnchainSats > 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for positive on-chain balance: last_balances=%+v last_err=%v", lastBalances, lastErr)
}

func waitForBestBlock(t *testing.T, backend *ldk.LDK, minHeight uint32, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastState ldk.DebugState
	var lastErr error
	for time.Now().Before(deadline) {
		if err := backend.SyncWallets(); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		state, err := backend.DebugState()
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		lastState = state
		if state.BestBlockHeight >= minHeight {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for best block >= %d: last_state=%+v last_err=%v", minHeight, lastState, lastErr)
}

func openChannelWithRetry(t *testing.T, backend *ldk.LDK, pubkey string, endpoint string, amount uint64, pushMsat uint64, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		if err := backend.SyncWallets(); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err := backend.OpenChannelWithPush(pubkey, endpoint, amount, pushMsat); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out opening channel: %w", lastErr)
}

func waitForChannelState(t *testing.T, backend *ldk.LDK, pubkey string, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := backend.SyncWallets(); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		summaries, err := backend.ChannelSummaries()
		if err == nil {
			for _, summary := range summaries {
				if summary.CounterpartyPub == pubkey && (summary.State == "pending" || summary.State == "active") {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for channel state for %s", pubkey)
}

func waitForBootstrapReady(t *testing.T, backend *ldk.LDK, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastBalances ldk.LDKBalances
	var lastErr error
	for time.Now().Before(deadline) {
		if err := backend.SyncWallets(); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		balances, err := backend.Balances()
		if err == nil {
			lastBalances = balances
		} else {
			lastErr = err
		}
		if err == nil && balances.AvailableOnchainSats > 0 && balances.LightningSats > 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for positive on-chain and lightning balances: last_balances=%+v last_err=%v", lastBalances, lastErr)
}
