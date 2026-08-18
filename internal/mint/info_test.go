package mint

import (
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestInfoReportsBolt11Availability(t *testing.T) {
	for _, test := range []struct {
		name         string
		backend      lightning.LightningBackend
		pegOutOnly   bool
		mintDisabled bool
		meltDisabled bool
	}{
		{name: "Strike", backend: lightning.Strike{}, pegOutOnly: false, mintDisabled: true, meltDisabled: true},
		{name: "supported backend", backend: lightning.FakeWallet{}, pegOutOnly: false, mintDisabled: false, meltDisabled: false},
		{name: "peg out only", backend: lightning.FakeWallet{}, pegOutOnly: true, mintDisabled: true, meltDisabled: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			mint := Mint{
				Config:           utils.Config{PEG_OUT_ONLY: test.pegOutOnly}, //nolint:exhaustruct
				LightningBackend: test.backend,
			} //nolint:exhaustruct
			info := mint.Info()

			if got := nutDisabled(t, info.Nuts, "4"); got != test.mintDisabled {
				t.Fatalf("NUT-4 disabled = %t, want %t", got, test.mintDisabled)
			}
			if got := nutDisabled(t, info.Nuts, "5"); got != test.meltDisabled {
				t.Fatalf("NUT-5 disabled = %t, want %t", got, test.meltDisabled)
			}
		})
	}
}

func nutDisabled(t *testing.T, nuts map[string]any, nut string) bool {
	t.Helper()
	info, ok := nuts[nut].(cashu.SwapMintInfo)
	if !ok || info.Disabled == nil {
		t.Fatalf("NUT-%s has no disabled flag", nut)
	}
	return *info.Disabled
}
