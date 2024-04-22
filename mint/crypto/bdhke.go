package crypto

import "github.com/decred/dcrd/dcrec/secp256k1/v4"

func SignBlindedMessage(B_ *secp256k1.PublicKey, k *secp256k1.PrivateKey) *secp256k1.PublicKey {
	var bpoint, result secp256k1.JacobianPoint
	B_.AsJacobian(&bpoint)

	secp256k1.ScalarMultNonConst(&k.Key, &bpoint, &result)
	result.ToAffine()
	C_ := secp256k1.NewPublicKey(&result.X, &result.Y)

	return C_
}
