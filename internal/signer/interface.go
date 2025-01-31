package signer

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	gonutsC "github.com/elnosh/gonuts/cashu"
	"github.com/lescuer97/nutmix/api/cashu"
)

type Signer interface {
	GetKeys() (GetKeysetsResponse, error)
	GetKeysById(id string) (GetKeysResponse, error)
	GetActiveKeys() (GetKeysResponse, error)
	GetKeysByUnit(unit cashu.Unit) ([]cashu.Keyset, error)
	GetPubkey() (*secp256k1.PublicKey, error)

	RotateKeyset(unit cashu.Unit, fee uint) error

	VerifyProofs(proofs gonutsC.Proofs, blindMessages gonutsC.BlindedMessages, unit cashu.Unit) error
	SignBlindMessages(messages gonutsC.BlindedMessages, unit cashu.Unit) (gonutsC.BlindedSignatures, error)
}
