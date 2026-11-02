//nolint:exhaustruct // Test fixtures intentionally specify only relevant fields.
package mint

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestBATReservationRejectsMissingPointAndFieldsBeforeDependencies(t *testing.T) {
	m := &Mint{}
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	for _, proof := range []cashu.AuthProof{
		{Id: "auth", Secret: "s"},
		{C: cashu.WrappedPublicKey{PublicKey: pub}, Secret: "s"},
		{C: cashu.WrappedPublicKey{PublicKey: pub}, Secret: "s"},
		{C: cashu.WrappedPublicKey{PublicKey: pub}, Id: "auth"},
	} {
		if _, err := m.ReserveAuthBlindToken(proof); !errors.Is(err, cashu.ErrInvalidAuthToken) {
			t.Fatalf("ReserveAuthBlindToken(%+v)=%v", proof, err)
		}
	}
}
