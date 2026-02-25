package admin

import (
	"fmt"
	"strings"

	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

type ldkNetworkSummary struct {
	TotalPeers     int
	ActivePeers    int
	TotalChannels  int
	ActiveChannels int
}

func mapLdkChannelStateLabel(state string) string {
	switch state {
	case "active":
		return "Active"
	case "offline":
		return "Offline"
	case "closing":
		return "Closing"
	default:
		return "Pending"
	}
}

func canCooperativeClose(channel ldk.LDKChannelSummary) bool {
	return channel.State == "active" && channel.PeerConnected
}

func canForceClose(channel ldk.LDKChannelSummary) bool {
	return channel.State == "offline" && !channel.PeerConnected
}

func findLdkChannelByID(channels []ldk.LDKChannelSummary, channelID string) (ldk.LDKChannelSummary, error) {
	if strings.TrimSpace(channelID) == "" {
		return ldk.LDKChannelSummary{}, fmt.Errorf("channel id is required")
	}

	for _, channel := range channels {
		if channel.ChannelID == channelID {
			return channel, nil
		}
	}

	return ldk.LDKChannelSummary{}, fmt.Errorf("channel not found")
}

func findLdkChannelForAction(channels []ldk.LDKChannelSummary, channelID string, counterpartyPub string) (ldk.LDKChannelSummary, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return ldk.LDKChannelSummary{}, fmt.Errorf("channel id is required")
	}

	counterpartyPub = strings.TrimSpace(counterpartyPub)
	if counterpartyPub == "" {
		return ldk.LDKChannelSummary{}, fmt.Errorf("counterparty public key is required")
	}

	channel, err := findLdkChannelByID(channels, channelID)
	if err != nil {
		return ldk.LDKChannelSummary{}, err
	}
	if channel.CounterpartyPub != counterpartyPub {
		return ldk.LDKChannelSummary{}, fmt.Errorf("channel details are stale, refresh and try again")
	}

	return channel, nil
}

func validateCooperativeClose(channel ldk.LDKChannelSummary) error {
	switch channel.State {
	case "closing":
		return fmt.Errorf("channel close is already in progress")
	case "pending":
		return fmt.Errorf("channel is still pending and cannot be closed yet")
	case "offline":
		return fmt.Errorf("channel peer must be connected before starting a cooperative close")
	}
	if !canCooperativeClose(channel) {
		return fmt.Errorf("channel peer must be connected before starting a cooperative close")
	}
	return nil
}

func validateForceClose(channel ldk.LDKChannelSummary) error {
	switch channel.State {
	case "closing":
		return fmt.Errorf("channel close is already in progress")
	case "pending":
		return fmt.Errorf("channel is still pending and cannot be force closed yet")
	}
	if !canForceClose(channel) {
		return fmt.Errorf("force close is only available while the channel is offline")
	}
	return nil
}

func mapCloseChannelError(err error, force bool) string {
	lower := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lower, "connected") || strings.Contains(lower, "peer"):
		return "Channel peer must be connected before starting a cooperative close"
	case strings.Contains(lower, "not found") || strings.Contains(lower, "unknown"):
		return "Channel not found"
	default:
		if force {
			return "Unable to start force close"
		}
		return "Unable to start cooperative close"
	}
}

func displayLdkValidationError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if message == "" {
		return ""
	}
	runes := []rune(message)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func balancePercents(local, remote uint64) (uint8, uint8) {
	total := local + remote
	if total == 0 {
		return 0, 0
	}

	localPct := uint8((local * 100) / total)
	remotePct := 100 - localPct
	return localPct, remotePct
}

func mapLdkNetworkSummary(peers []ldk.LDKPeerSummary, channels []ldk.LDKChannelSummary) ldkNetworkSummary {
	summary := ldkNetworkSummary{
		TotalPeers:     len(peers),
		ActivePeers:    0,
		TotalChannels:  len(channels),
		ActiveChannels: 0,
	}

	for _, peer := range peers {
		if peer.IsConnected {
			summary.ActivePeers++
		}
	}

	for _, channel := range channels {
		if channel.State == "active" {
			summary.ActiveChannels++
		}
	}

	return summary
}

func mapLdkChannelRows(channels []ldk.LDKChannelSummary) []templates.LdkChannelRow {
	rows := make([]templates.LdkChannelRow, 0, len(channels))
	for _, channel := range channels {
		localPct, remotePct := balancePercents(channel.LocalBalanceSats, channel.RemoteBalanceSats)
		rows = append(rows, templates.LdkChannelRow{
			ChannelID:         channel.ChannelID,
			CounterpartyLabel: channel.CounterpartyLabel,
			CounterpartyPub:   channel.CounterpartyPub,
			LocalBalance:      templates.FormatNumber(channel.LocalBalanceSats) + " sats",
			RemoteBalance:     templates.FormatNumber(channel.RemoteBalanceSats) + " sats",
			LocalBalanceSats:  channel.LocalBalanceSats,
			RemoteBalanceSats: channel.RemoteBalanceSats,
			TotalBalanceSats:  channel.LocalBalanceSats + channel.RemoteBalanceSats,
			LocalBalancePct:   localPct,
			RemoteBalancePct:  remotePct,
			StateLabel:        mapLdkChannelStateLabel(channel.State),
			CanClose:          canCooperativeClose(channel),
			CanForceClose:     canForceClose(channel),
		})
	}
	return rows
}
