package mint

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/elnosh/gonuts/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
)

type authTestDB struct {
	mockdb.MockDB
}

// MockDB compares public-key pointers; PostgreSQL compares their encoded values.
func (db *authTestDB) GetProofsFromSecretCurve(_ pgx.Tx, ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	var proofs cashu.Proofs
	for _, proof := range db.Proofs {
		for _, y := range ys {
			if proof.Y.ToHex() == y.ToHex() {
				proofs = append(proofs, proof)
			}
		}
	}
	return proofs, nil
}

func TestVerifyAuthBlindToken(t *testing.T) {
	for _, tc := range []struct {
		wantErr  error
		name     string
		unit     cashu.Unit
		tampered bool
	}{
		{name: "sat proof rejected", unit: cashu.Sat, tampered: false, wantErr: cashu.ErrKeysetNotKnow},
		{name: "auth proof accepted", unit: cashu.AUTH, tampered: false, wantErr: nil},
		{name: "tampered auth proof rejected", unit: cashu.AUTH, tampered: true, wantErr: cashu.ErrInvalidProof},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
			db := new(authTestDB)
			sig, err := localsigner.SetupLocalSigner(db)
			if err != nil {
				t.Fatal(err)
			}
			if err := sig.RotateKeyset(cashu.AUTH, 0, 0); err != nil {
				t.Fatal(err)
			}
			keys, err := sig.GetActiveKeys()
			if tc.unit == cashu.AUTH {
				keys, err = sig.GetAuthActiveKeys()
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(keys.Keysets) != 1 || keys.Keysets[0].Unit != tc.unit.String() {
				t.Fatalf("unexpected keysets: %+v", keys.Keysets)
			}
			keyset := keys.Keysets[0]
			pubBytes, err := hex.DecodeString(keyset.Keys[1])
			if err != nil {
				t.Fatal(err)
			}
			pubkey, err := secp256k1.ParsePubKey(pubBytes)
			if err != nil {
				t.Fatal(err)
			}
			r, err := secp256k1.GeneratePrivateKey()
			if err != nil {
				t.Fatal(err)
			}
			blinded, _, err := crypto.BlindMessage(tc.name, r)
			if err != nil {
				t.Fatal(err)
			}
			signatures, _, err := sig.SignBlindMessages([]cashu.BlindedMessage{{
				Id: keyset.Id, Amount: 1, B_: cashu.WrappedPublicKey{PublicKey: blinded}, Witness: "",
			}})
			if err != nil {
				t.Fatal(err)
			}
			proof := cashu.AuthProof{
				Id: keyset.Id, Amount: 1, Secret: tc.name,
				C: cashu.WrappedPublicKey{PublicKey: crypto.UnblindSignature(signatures[0].C_.PublicKey, r, pubkey)},
			}
			if tc.tampered {
				proof.C.PublicKey = pubkey
			}
			m := new(Mint)
			m.MintDB = db
			m.Signer = &sig
			err = m.VerifyAuthBlindToken(proof)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyAuthBlindToken() = %v, want %v", err, tc.wantErr)
			}
			if tc.unit == cashu.Sat && len(db.Proofs) != 0 {
				t.Fatal("rejected sat proof was saved")
			}
			if tc.wantErr != nil {
				return
			}
			if len(db.Proofs) != 1 || db.Proofs[0].State != cashu.PROOF_SPENT {
				t.Fatalf("accepted auth proof not marked spent: %+v", db.Proofs)
			}
			if err := m.VerifyAuthBlindToken(proof); err == nil || err.Error() != "authProof already used" {
				t.Fatalf("reused auth proof: got %v, want authProof already used", err)
			}
		})
	}
}
