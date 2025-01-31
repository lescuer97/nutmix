package signer

import "github.com/lescuer97/nutmix/api/cashu"

type Signer interface {
	GetKeys() (GetKeysetsResponse, error)
	GetKeysById(id string) (GetKeysResponse, error)
	GetActiveKeys() (GetKeysResponse, error)
	GetKeysByUnit(unit cashu.Unit) ([]cashu.Keyset, error)

	RotateKeyset(unit cashu.Unit, fee uint) error
	GetSignerPubkey() (string, error)

	VerifyProofs(proofs []cashu.Proof, blindMessages []cashu.BlindedMessage) error
	SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error)
}
