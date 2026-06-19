package ldk

import (
	"fmt"
	"sort"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
)

type LDKChannelSummary struct {
	CounterpartyLabel string
	CounterpartyPub   string
	ChannelID         string
	State             string
	LocalBalanceSats  uint64
	RemoteBalanceSats uint64
	PeerConnected     bool
}

type LDKPeerSummary struct {
	NodePub     string
	Address     string
	IsPersisted bool
	IsConnected bool
}

func (l *LDK) OpenChannel(nodeID string, address string, sats uint64) error {
	return l.OpenChannelWithPush(nodeID, address, sats, 0)
}

func (l *LDK) OpenChannelWithPush(nodeID string, address string, sats uint64, pushMsat uint64) error {
	node, err := l.getNode()
	if err != nil {
		return err
	}

	config := ldk_node.ChannelConfig{
		ForwardingFeeProportionalMillionths: 100,
		ForwardingFeeBaseMsat:               100,
		CltvExpiryDelta:                     1600,
		MaxDustHtlcExposure:                 ldk_node.MaxDustHtlcExposureFixedLimit{LimitMsat: 10000},
		ForceCloseAvoidanceMaxFeeSatoshis:   1000,
		AcceptUnderpayingHtlcs:              false,
	}

	pushToAmount := pushMsat
	_, err = node.OpenChannel(nodeID, address, sats, &pushToAmount, &config)
	if err != nil {
		return fmt.Errorf("node.OpenChannel(...): %w", err)
	}

	return nil
}

func (l *LDK) CloseChannel(channelID string, counterpartyPub string) error {
	node, err := l.getNode()
	if err != nil {
		return err
	}

	err = node.CloseChannel(channelID, counterpartyPub)
	if err != nil {
		return fmt.Errorf("node.CloseChannel(...): %w", err)
	}

	return nil
}

func (l *LDK) ForceCloseChannel(channelID string, counterpartyPub string) error {
	node, err := l.getNode()
	if err != nil {
		return err
	}

	err = node.ForceCloseChannel(channelID, counterpartyPub, nil)
	if err != nil {
		return fmt.Errorf("node.ForceCloseChannel(...): %w", err)
	}

	return nil
}

func (l *LDK) ChannelSummaries() ([]LDKChannelSummary, error) {
	node, err := l.getNode()
	if err != nil {
		return nil, err
	}

	channels := node.ListChannels()
	peers := node.ListPeers()
	connectedPeers := make(map[string]bool, len(peers))
	for _, peer := range peers {
		connectedPeers[peer.NodeId] = peer.IsConnected
	}

	lookupPeerConnection := func(pub string) bool {
		return connectedPeers[pub]
	}

	return mapChannelSummaries(channels, lookupPeerConnection), nil
}

func mapChannelSummaries(channels []ldk_node.ChannelDetails, isPeerConnected func(pub string) bool) []LDKChannelSummary {
	summaries := make([]LDKChannelSummary, 0, len(channels))
	for _, channel := range channels {
		peerConnected := false
		if isPeerConnected != nil {
			peerConnected = isPeerConnected(channel.CounterpartyNodeId)
		}
		summaries = append(summaries, LDKChannelSummary{
			ChannelID:         channel.UserChannelId,
			State:             deriveChannelState(channel, peerConnected),
			PeerConnected:     peerConnected,
			CounterpartyLabel: channel.CounterpartyNodeId,
			CounterpartyPub:   channel.CounterpartyNodeId,
			LocalBalanceSats:  channel.OutboundCapacityMsat / 1000,
			RemoteBalanceSats: channel.InboundCapacityMsat / 1000,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CounterpartyPub < summaries[j].CounterpartyPub
	})

	return summaries
}

func deriveChannelState(channel ldk_node.ChannelDetails, peerConnected bool) string {
	switch {
	case channel.IsUsable && channel.IsChannelReady && peerConnected:
		return "active"
	case !channel.IsChannelReady:
		return "pending"
	case channel.IsChannelReady && !channel.IsUsable && peerConnected:
		return "closing"
	case channel.IsChannelReady && !peerConnected:
		return "offline"
	default:
		return "pending"
	}
}

func (l *LDK) PeerSummaries() ([]LDKPeerSummary, error) {
	node, err := l.getNode()
	if err != nil {
		return nil, err
	}
	return mapPeerSummaries(node.ListPeers()), nil
}

func mapPeerSummaries(peers []ldk_node.PeerDetails) []LDKPeerSummary {
	summaries := make([]LDKPeerSummary, 0, len(peers))
	for _, peer := range peers {
		if !peer.IsConnected {
			continue
		}
		summaries = append(summaries, LDKPeerSummary{
			NodePub:     peer.NodeId,
			Address:     peer.Address,
			IsPersisted: peer.IsPersisted,
			IsConnected: peer.IsConnected,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].NodePub < summaries[j].NodePub
	})

	return summaries
}
