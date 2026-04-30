package ldk

import (
	"testing"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
)

func TestMapChannelSummariesFallbackAndSort(t *testing.T) {
	channels := []ldk_node.ChannelDetails{
		newTestChannelDetails("03ff", "chan-2", 4500, 7000, true, true),
		newTestChannelDetails("02aa", "chan-1", 2500, 1000, true, true),
	}

	got := mapChannelSummaries(channels, func(pub string) bool {
		return pub == "03ff"
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(got))
	}

	if got[0].CounterpartyPub != "02aa" || got[1].CounterpartyPub != "03ff" {
		t.Fatalf("expected sorted by pubkey, got %+v", got)
	}

	if got[0].CounterpartyLabel != "02aa" {
		t.Fatalf("expected counterparty label to come from channel details, got %q", got[0].CounterpartyLabel)
	}
	if got[1].CounterpartyLabel != "03ff" {
		t.Fatalf("expected counterparty label to use node id, got %q", got[1].CounterpartyLabel)
	}
	if got[1].LocalBalanceSats != 4 || got[1].RemoteBalanceSats != 7 {
		t.Fatalf("expected msat to sats conversion, got %+v", got[1])
	}
	if got[0].ChannelID != "chan-1" || got[1].ChannelID != "chan-2" {
		t.Fatalf("expected user channel ids to be preserved, got %+v", got)
	}
	if got[0].State != "offline" || got[1].State != "active" {
		t.Fatalf("expected raw states to include offline/active, got %+v", got)
	}
	if got[0].PeerConnected || !got[1].PeerConnected {
		t.Fatalf("expected peer connection flags to be mapped, got %+v", got)
	}
}

func TestMapChannelSummariesRawStates(t *testing.T) {
	tests := []struct {
		name          string
		channel       ldk_node.ChannelDetails
		peerConnected bool
		wantState     string
	}{
		{
			name:          "active",
			channel:       newTestChannelDetails("02aa", "chan-active", 2000, 3000, true, true),
			peerConnected: true,
			wantState:     "active",
		},
		{
			name:          "offline",
			channel:       newTestChannelDetails("02bb", "chan-offline", 2000, 3000, true, true),
			peerConnected: false,
			wantState:     "offline",
		},
		{
			name:          "pending",
			channel:       newTestChannelDetails("02cc", "chan-pending", 2000, 3000, false, false),
			peerConnected: true,
			wantState:     "pending",
		},
		{
			name:          "pending while disconnected stays pending",
			channel:       newTestChannelDetails("02ce", "chan-pending-disconnected", 2000, 3000, false, false),
			peerConnected: false,
			wantState:     "pending",
		},
		{
			name:          "closing",
			channel:       newTestChannelDetails("02dd", "chan-closing", 2000, 3000, true, false),
			peerConnected: true,
			wantState:     "closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapChannelSummaries([]ldk_node.ChannelDetails{tt.channel}, func(string) bool {
				return tt.peerConnected
			})
			if len(got) != 1 {
				t.Fatalf("expected 1 summary, got %d", len(got))
			}
			if got[0].State != tt.wantState {
				t.Fatalf("expected state %q, got %q", tt.wantState, got[0].State)
			}
		})
	}
}

func newTestChannelDetails(pub string, channelID string, outboundMsat uint64, inboundMsat uint64, ready bool, usable bool) ldk_node.ChannelDetails {
	var details ldk_node.ChannelDetails
	details.UserChannelId = channelID
	details.CounterpartyNodeId = pub
	details.OutboundCapacityMsat = outboundMsat
	details.InboundCapacityMsat = inboundMsat
	details.IsChannelReady = ready
	details.IsUsable = usable
	return details
}
