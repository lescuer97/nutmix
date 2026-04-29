package ldk

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
)

type PaymentResponse = lightning.PaymentResponse
type PaymentStatus = lightning.PaymentStatus
type FeesResponse = lightning.FeesResponse
type InvoiceResponse = lightning.InvoiceResponse
type Backend = lightning.Backend

type Options struct {
	StorageDir string
}

const (
	SETTLED = lightning.SETTLED
	FAILED  = lightning.FAILED
	PENDING = lightning.PENDING
	UNKNOWN = lightning.UNKNOWN
	LDKNODE = lightning.LDKNODE
)

type LDK struct {
	node    *ldk_node.Node
	db      database.MintDB
	doneCh  chan struct{}
	network string
	options Options

	mu      sync.Mutex
	started bool
}

func NewLdk(ctx context.Context, db database.MintDB, network string) (*LDK, error) {
	return NewLdkWithOptions(ctx, db, network, Options{StorageDir: ""})
}

func NewLdkWithOptions(ctx context.Context, db database.MintDB, network string, options Options) (*LDK, error) {
	ldk := NewConfigBackendWithOptions(db, network, options)

	err := ldk.InitNode(ctx)
	if err != nil {
		return nil, fmt.Errorf("ldk.InitNode(). %w", err)
	}
	err = ldk.SpinUp()
	if err != nil {
		return nil, fmt.Errorf("could not start up ldk node . %w", err)
	}

	return ldk, nil
}

func NewConfigBackend(db database.MintDB, network string) (*LDK, error) {
	return NewConfigBackendWithOptions(db, network, Options{StorageDir: ""}), nil
}

func NewConfigBackendWithOptions(db database.MintDB, network string, options Options) *LDK {
	backend := &LDK{
		node:    nil,
		db:      db,
		doneCh:  nil,
		network: network,
		options: options,
		mu:      sync.Mutex{},
		started: false,
	}
	return backend
}

func (l *LDK) storageDir() string {
	return l.options.StorageDir
}

func (l *LDK) InitNode(ctx context.Context) error {
	if l == nil {
		return fmt.Errorf("ldk backend is nil")
	}

	seedMnemonic, ldkStorage, network, config, err := l.prepareInitConfig(ctx)
	if err != nil {
		return fmt.Errorf("l.prepareInitConfig(ctx): %w", err)
	}

	builder := ldk_node.NewBuilder()
	builder.SetNetwork(network)
	// if err := builder.SetListeningAddresses([]ldk_node.SocketAddress{"127.0.0.1:39735"}); err != nil {
	// 	return fmt.Errorf("builder.SetListeningAddresses(...): %w", err)
	// }
	switch config.ChainSourceType {
	case ChainSourceElectrum:
		builder.SetChainSourceElectrum(config.ElectrumServerURL, nil)
	case ChainSourceEsplora:
		builder.SetChainSourceEsplora(config.EsploraServerURL, forcedEsploraSyncConfig())
	case ChainSourceBitcoind:
		builder.SetChainSourceBitcoindRpc(
			config.Rpc.Address,
			config.Rpc.Port,
			config.Rpc.Username,
			config.Rpc.Password,
		)
	default:
		return fmt.Errorf("unsupported chain source type %q", config.ChainSourceType)
	}
	builder.SetGossipSourceP2p()

	nodeEntropy := ldk_node.NodeEntropyFromBip39Mnemonic(seedMnemonic, nil)
	slog.Debug("building ldk node")

	builder.SetStorageDirPath(ldkStorage)
	node, err := builder.Build(nodeEntropy)
	if err != nil {
		return fmt.Errorf("could not Create ldk-node. %w", err)
	}

	l.node = node
	return nil
}

func forcedEsploraSyncConfig() *ldk_node.EsploraSyncConfig {
	return &ldk_node.EsploraSyncConfig{
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

	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return nil
	}

	l.doneCh = make(chan struct{})
	l.started = true

	node := l.node
	doneCh := l.doneCh
	l.mu.Unlock()

	slog.Info("Starting to run ldk node")
	if err := node.Start(); err != nil {
		l.finishRun(doneCh)
		close(doneCh)
		return fmt.Errorf("node.Start(): %w", err)
	}
	slog.Info("ldk node started")

	go l.run(node, doneCh)
	return nil
}

func (l *LDK) Stop() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	if !l.started || l.node == nil {
		l.mu.Unlock()
		return nil
	}

	node := l.node
	doneCh := l.doneCh
	l.mu.Unlock()

	err := node.Stop()
	if doneCh != nil {
		if node.Status().IsRunning {
			<-doneCh
		} else {
			l.finishRun(doneCh)
		}
	}

	return err
}

func (l *LDK) run(node *ldk_node.Node, doneCh chan struct{}) {
	defer close(doneCh)
	defer l.finishRun(doneCh)

	defer node.Destroy()
	defer slog.Info("ldk node stopped")

	for node.Status().IsRunning {
		_ = node.NextEventAsync()

		if err := node.EventHandled(); err != nil {
			if !node.Status().IsRunning {
				return
			}
			slog.Error("could not handle ldk event", slog.Any("error", err))
		}
	}
}

func (l *LDK) finishRun(doneCh chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.doneCh != doneCh {
		return
	}
	l.started = false
	l.doneCh = nil
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
