package ldk

import (
	"context"
	"testing"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/lightning"
)

func TestStatusReturnsOfflineWhenNodeIsNotInitialized(t *testing.T) {
	status, err := (&LDK{}).Status(context.Background())
	if err == nil {
		t.Fatal("expected uninitialized LDK status error")
	}
	if status != lightning.OFFLINE_STATUS {
		t.Fatalf("status = %q, want %q", status, lightning.OFFLINE_STATUS)
	}
}

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
