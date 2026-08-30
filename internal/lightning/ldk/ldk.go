package ldk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
)

type PaymentResponse = lightning.PaymentResponse
type PaymentStatus = lightning.PaymentStatus
type FeesResponse = lightning.FeesResponse
type InvoiceResponse = lightning.InvoiceResponse
type Backend = lightning.Backend

var _ lightning.LightningBackend = (*LDK)(nil)

const (
	SETTLED = lightning.SETTLED
	FAILED  = lightning.FAILED
	PENDING = lightning.PENDING
	UNKNOWN = lightning.UNKNOWN
	LDKNODE = lightning.LDKNODE

	inFlightPaymentWaitTimeout  = 90 * time.Second
	inFlightPaymentPollInterval = time.Second
)

type StopMode uint8

const (
	StopImmediately StopMode = iota
	WaitForInFlightPayments
)

var ErrInFlightBolt11Payments = errors.New("outbound Bolt 11 payments are still in flight")

type LdkConfig struct {
	Network    string
	StorageDir string
	TorOnly    bool
	NoOutgoing bool
}

type LDK struct {
	db       database.MintDB
	node     *ldk_node.Node
	config   LdkConfig
	configMu sync.RWMutex
}

func NewLdk(ctx context.Context, db database.MintDB, config LdkConfig) (*LDK, error) {
	persistedConfig, err := GetPersistedConfig(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("GetPersistedConfig(ctx, db): %w", err)
	}

	ldk, err := NewConfigBackend(db, config)
	if err != nil {
		return nil, err
	}
	return startLDK(ctx, ldk, persistedConfig)
}

func NewLdkWithPersistedConfig(ctx context.Context, db database.MintDB, config LdkConfig, persistedConfig PersistedConfig) (*LDK, error) {
	normalizedConfig, err := normalizePersistedConfig(persistedConfig)
	if err != nil {
		return nil, fmt.Errorf("normalizePersistedConfig(persistedConfig): %w", err)
	}

	ldk, err := NewConfigBackend(db, config)
	if err != nil {
		return nil, err
	}

	return startLDK(ctx, ldk, normalizedConfig)
}

func startLDK(ctx context.Context, ldk *LDK, persistedConfig PersistedConfig) (*LDK, error) {
	err := ldk.initNode(ctx, persistedConfig)
	if err != nil {
		return nil, fmt.Errorf("ldk.InitNode(). %w", err)
	}
	err = ldk.SpinUp()
	if err != nil {
		return nil, fmt.Errorf("could not start up ldk node . %w", err)
	}

	return ldk, nil
}

func NewConfigBackend(db database.MintDB, config LdkConfig) (*LDK, error) {
	return &LDK{
		node:     nil,
		db:       db,
		configMu: sync.RWMutex{},
		config:   config,
	}, nil
}

func (l *LDK) storageDir() string {
	return l.configSnapshot().StorageDir
}

func (l *LDK) configSnapshot() LdkConfig {
	if l == nil {
		return LdkConfig{
			Network:    "",
			StorageDir: "",
			TorOnly:    false,
			NoOutgoing: false,
		}
	}

	l.configMu.RLock()
	defer l.configMu.RUnlock()
	return l.config
}

func (l *LDK) setTorOnly(torOnly bool) {
	l.configMu.Lock()
	l.config.TorOnly = torOnly
	l.configMu.Unlock()
}

func (l *LDK) setNoOutgoing(noOutgoing bool) {
	l.configMu.Lock()
	l.config.NoOutgoing = noOutgoing
	l.configMu.Unlock()
}

func (l *LDK) checkOutgoingAllowed() error {
	if l == nil {
		return fmt.Errorf("ldk backend is nil")
	}
	if l.configSnapshot().NoOutgoing {
		return cashu.ErrMeltingDisabled
	}
	return nil
}

func (l *LDK) InitNode(ctx context.Context) error {
	if l == nil {
		return fmt.Errorf("ldk backend is nil")
	}

	persistedConfig, err := GetPersistedConfig(ctx, l.db)
	if err != nil {
		return fmt.Errorf("GetPersistedConfig(ctx, l.db): %w", err)
	}

	return l.initNode(ctx, persistedConfig)
}

func (l *LDK) initNode(ctx context.Context, persistedConfig PersistedConfig) error {
	seedMnemonic, ldkStorage, network, config, err := l.prepareInitConfig(persistedConfig)
	if err != nil {
		return fmt.Errorf("l.prepareInitConfig(persistedConfig): %w", err)
	}

	builder := ldk_node.NewBuilder()
	builder.SetNetwork(network)
	switch config.ChainSourceType {
	case ChainSourceElectrum:
		builder.SetChainSourceElectrum(config.ElectrumServerURL, &ldk_node.ElectrumSyncConfig{
			FullScanStopGap:     20,
			ForceWalletFullScan: false,
			BackgroundSyncConfig: &ldk_node.BackgroundSyncConfig{
				OnchainWalletSyncIntervalSecs:   80,
				LightningWalletSyncIntervalSecs: 30,
				FeeRateCacheUpdateIntervalSecs:  600,
			},
			TimeoutsConfig: ldk_node.SyncTimeoutsConfig{
				OnchainWalletSyncTimeoutSecs:   60,
				LightningWalletSyncTimeoutSecs: 30,
				FeeRateCacheUpdateTimeoutSecs:  10,
				TxBroadcastTimeoutSecs:         10,
				PerRequestTimeoutSecs:          10,
			}})
	case ChainSourceEsplora:
		builder.SetChainSourceEsplora(config.EsploraServerURL, forcedEsploraSyncConfig())
	case ChainSourceBitcoind:
		builder.SetChainSourceBitcoindRpc(
			config.Rpc.Address,
			config.Rpc.Port,
			config.Rpc.Username,
			config.Rpc.Password,
			nil,
		)
	default:
		return fmt.Errorf("unsupported chain source type %q", config.ChainSourceType)
	}
	builder.SetGossipSourceP2p()
	if config.TorProxyAddress != nil {
		if err := builder.SetTorConfig(ldk_node.TorConfig{ProxyAddress: *config.TorProxyAddress}); err != nil {
			return fmt.Errorf("set tor config: %w", err)
		}
	}

	nodeEntropy := ldk_node.NodeEntropyFromBip39Mnemonic(seedMnemonic, nil)
	slog.Debug("building ldk node")

	builder.SetStorageDirPath(ldkStorage)
	node, err := builder.Build(nodeEntropy)
	if err != nil {
		return fmt.Errorf("could not Create ldk-node. %w", err)
	}
	l.node = node
	if config.TorOnly {
		listeningAddresses := node.ListeningAddresses()
		log.Printf("listeningAddresses: %+v", listeningAddresses)
		if listeningAddresses != nil {
			if err := validateTorOnlyListeningAddresses(*listeningAddresses); err != nil {
				validationErr := fmt.Errorf("validate tor-only listening addresses: %w", err)
				if stopErr := node.Stop(); stopErr != nil {
					validationErr = errors.Join(validationErr, fmt.Errorf("stop invalid LDK node: %w", stopErr))
				}
				l.node = nil
				return validationErr
			}
		}
	}

	l.setTorOnly(config.TorOnly)
	return nil
}

func forcedEsploraSyncConfig() *ldk_node.EsploraSyncConfig {
	return &ldk_node.EsploraSyncConfig{
		FullScanStopGap:     20,
		ForceWalletFullScan: false,
		BackgroundSyncConfig: &ldk_node.BackgroundSyncConfig{
			OnchainWalletSyncIntervalSecs:   80,
			LightningWalletSyncIntervalSecs: 30,
			FeeRateCacheUpdateIntervalSecs:  600,
		},
		TimeoutsConfig: ldk_node.SyncTimeoutsConfig{
			OnchainWalletSyncTimeoutSecs:   60,
			LightningWalletSyncTimeoutSecs: 30,
			FeeRateCacheUpdateTimeoutSecs:  10,
			TxBroadcastTimeoutSecs:         10,
			PerRequestTimeoutSecs:          10,
		},
	}
}

func (l *LDK) SpinUp() error {
	if l.node == nil {
		return fmt.Errorf("ldk node is not spun up")
	}

	slog.Info("Starting to run ldk node")
	if err := l.node.Start(); err != nil {
		errStop := l.node.Stop()
		if errStop != nil {
			return fmt.Errorf("node.Stop(): %w", errStop)
		}
		return fmt.Errorf("node.Start(): %w", err)
	}
	slog.Info("ldk node started")

	go l.run()
	return nil
}

func (l *LDK) Stop(ctx context.Context, mode StopMode) error {
	if mode != StopImmediately && mode != WaitForInFlightPayments {
		return fmt.Errorf("unknown LDK stop mode: %d", mode)
	}
	if l == nil {
		return nil
	}

	if l.node == nil {
		return nil
	}

	if mode == WaitForInFlightPayments {
		l.setNoOutgoing(true)
		stopCtx, cancel := context.WithTimeoutCause(ctx, inFlightPaymentWaitTimeout, ErrInFlightBolt11Payments)
		err := waitForInFlightPayments(stopCtx, l.node.ListPayments)
		cancel()
		if err != nil {
			l.setNoOutgoing(false)
			return fmt.Errorf("wait for in-flight payments: %w", err)
		}
	}

	err := l.node.Stop()
	if err != nil {
		if mode == WaitForInFlightPayments {
			l.setNoOutgoing(false)
		}
		return fmt.Errorf("l.node.Stop(). %w", err)
	}

	return nil
}

func waitForInFlightPayments(ctx context.Context, listPayments func() []ldk_node.PaymentDetails) error {
	ticker := time.NewTicker(inFlightPaymentPollInterval)
	defer ticker.Stop()

	for hasPendingOutgoingBolt11Payments(listPayments()) {
		select {
		case <-ctx.Done():
			if !hasPendingOutgoingBolt11Payments(listPayments()) {
				return nil
			}
			return context.Cause(ctx)
		case <-ticker.C:
		}
	}

	return nil
}

func (l *LDK) Status(_ context.Context) (lightning.NodeStatus, error) {
	node, err := l.getNode()
	if err != nil {
		return lightning.OFFLINE_STATUS, err
	}
	if node.Status().IsRunning {
		return lightning.ONLINE_STATUS, nil
	}
	return lightning.STOPPED_STATUS, nil
}

func (l *LDK) run() {
	for l.node.Status().IsRunning {
		_ = l.node.NextEventAsync()

		if err := l.node.EventHandled(); err != nil {
			if !l.node.Status().IsRunning {
				return
			}
			slog.Error("could not handle ldk event", slog.Any("error", err))
		}
	}
}

func convertChaninParamsToLdkNetwork(param chaincfg.Params) (ldk_node.Network, error) {
	switch param.Net {
	case wire.MainNet:
		return ldk_node.NetworkBitcoin, nil
		// testnet actually represents regtest
	case wire.TestNet:
		return ldk_node.NetworkRegtest, nil
	case wire.TestNet3:
		return ldk_node.NetworkTestnet, nil
	case wire.SigNet:
		return ldk_node.NetworkSignet, nil
	default:
		return 999, fmt.Errorf("could parse network type")
	}
}

func (l *LDK) getNode() (*ldk_node.Node, error) {
	if l == nil {
		return nil, fmt.Errorf("ldk backend is nil")
	}
	if l.node == nil {
		return nil, fmt.Errorf("ldk node is not initialized")
	}
	return l.node, nil
}
