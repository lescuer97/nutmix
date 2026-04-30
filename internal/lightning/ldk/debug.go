package ldk

import (
	"fmt"
)

type DebugState struct {
	LatestLightningSyncTimestamp *uint64
	LatestOnchainSyncTimestamp   *uint64
	LatestFeeRateSyncTimestamp   *uint64
	NodeID                       string
	BestBlockHash                string
	ListeningAddresses           []string
	TotalOnchainSats             uint64
	AvailableOnchainSats         uint64
	LightningSats                uint64
	BestBlockHeight              uint32
	IsRunning                    bool
}

func (l *LDK) DebugState() (DebugState, error) {
	node, err := l.getNode()
	if err != nil {
		return DebugState{}, err
	}

	status := node.Status()
	balances := mapLDKBalances(node.ListBalances())
	listening := []string{}
	if addresses := node.ListeningAddresses(); addresses != nil {
		listening = make([]string, 0, len(*addresses))
		listening = append(listening, (*addresses)...)
	}

	return DebugState{
		NodeID:                       node.NodeId(),
		ListeningAddresses:           listening,
		IsRunning:                    status.IsRunning,
		BestBlockHeight:              status.CurrentBestBlock.Height,
		BestBlockHash:                fmt.Sprintf("%v", status.CurrentBestBlock.BlockHash),
		LatestLightningSyncTimestamp: cloneUint64(status.LatestLightningWalletSyncTimestamp),
		LatestOnchainSyncTimestamp:   cloneUint64(status.LatestOnchainWalletSyncTimestamp),
		LatestFeeRateSyncTimestamp:   cloneUint64(status.LatestFeeRateCacheUpdateTimestamp),
		TotalOnchainSats:             balances.TotalOnchainSats,
		AvailableOnchainSats:         balances.AvailableOnchainSats,
		LightningSats:                balances.LightningSats,
	}, nil
}

func cloneUint64(v *uint64) *uint64 {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}
