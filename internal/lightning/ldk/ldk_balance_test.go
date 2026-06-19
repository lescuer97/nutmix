package ldk

import (
	"testing"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
)

func TestMapLDKBalancesMapsOnchainAndLightning(t *testing.T) {
	input := ldk_node.BalanceDetails{
		TotalOnchainBalanceSats:            321,
		SpendableOnchainBalanceSats:        123,
		TotalAnchorChannelsReserveSats:     0,
		TotalLightningBalanceSats:          654,
		LightningBalances:                  nil,
		PendingBalancesFromChannelClosures: nil,
	}

	got := mapLDKBalances(input)

	if got.TotalOnchainSats != 321 {
		t.Fatalf("expected total on-chain sats 321, got %d", got.TotalOnchainSats)
	}
	if got.AvailableOnchainSats != 123 {
		t.Fatalf("expected available on-chain sats 123, got %d", got.AvailableOnchainSats)
	}
	if got.LightningSats != 654 {
		t.Fatalf("expected lightning sats 654, got %d", got.LightningSats)
	}
}
