package utils

import "testing"

func TestCanUseLiquidityManager(t *testing.T) {
	for _, test := range []struct {
		backend LightningBackend
		want    bool
	}{
		{backend: Strike, want: false},
		{backend: FAKE_WALLET, want: false},
		{backend: LNDGRPC, want: true},
		{backend: CLNGRPC, want: true},
		{backend: LNBITS, want: true},
	} {
		if got := CanUseLiquidityManager(test.backend); got != test.want {
			t.Fatalf("CanUseLiquidityManager(%q) = %t, want %t", test.backend, got, test.want)
		}
	}
}
