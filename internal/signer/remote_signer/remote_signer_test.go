package remotesigner

import (
	"errors"
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestRejectsNilBlindedMessage(t *testing.T) {
	messages := []cashu.BlindedMessage{{B_: cashu.WrappedPublicKey{PublicKey: nil}, Id: "", Witness: "", Amount: 0}}

	_, _, err := (&RemoteSigner{grpcClient: nil, activeKeysets: nil, keysets: nil, pubkey: nil}).SignBlindMessages(messages)
	if !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Errorf("SignBlindMessages error = %v, want ErrInvalidBlindMessage", err)
	}

	_, err = ConvertBlindedMessagedToGRPC(messages)
	if !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Errorf("ConvertBlindedMessagedToGRPC error = %v, want ErrInvalidBlindMessage", err)
	}
}
