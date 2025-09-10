package signer

import "github.com/lescuer97/nutmix/api/cashu"

type Signer interface {
	GetKeysets() (GetKeysetsResponse, error)
	GetKeysById(id string) (GetKeysResponse, error)
	GetActiveKeys() (GetKeysResponse, error)

	GetAuthKeys() (GetKeysetsResponse, error)
	GetAuthKeysById(id string) (GetKeysResponse, error)
	GetAuthActiveKeys() (GetKeysResponse, error)

	RotateKeyset(unit cashu.Unit, fee uint, expiry_limit uint) error
	GetSignerPubkey() (string, error)

	SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error)
	VerifyProofs(proofs []cashu.Proof) error
}
