package mint

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

type checkProofStateDB struct {
	database.MintDB
	proofs cashu.Proofs
	gotYs  []cashu.WrappedPublicKey
}

func (db *checkProofStateDB) GetTx(context.Context) (pgx.Tx, error)  { return nil, nil }
func (db *checkProofStateDB) Rollback(context.Context, pgx.Tx) error { return nil }
func (db *checkProofStateDB) Commit(context.Context, pgx.Tx) error   { return nil }
func (db *checkProofStateDB) GetProofsFromSecretCurve(_ pgx.Tx, ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	db.gotYs = ys
	return db.proofs, nil
}

func TestCheckProofState(t *testing.T) {
	key1, _ := btcec.PrivKeyFromBytes([]byte{1})
	key2, _ := btcec.PrivKeyFromBytes([]byte{2})
	key3, _ := btcec.PrivKeyFromBytes([]byte{3})
	y1 := cashu.WrappedPublicKey{PublicKey: key1.PubKey()}
	y2 := cashu.WrappedPublicKey{PublicKey: key2.PubKey()}
	y3 := cashu.WrappedPublicKey{PublicKey: key3.PubKey()}
	witness := "spent witness"
	db := &checkProofStateDB{proofs: cashu.Proofs{
		{Y: y1, State: cashu.PROOF_SPENT, Witness: witness},
		{Y: y2, State: cashu.PROOF_PENDING},
	}}
	m := Mint{MintDB: db} //nolint:exhaustruct

	got, err := CheckProofState(context.Background(), &m, []cashu.WrappedPublicKey{y1, y3, y2, y1})
	if err != nil {
		t.Fatal(err)
	}
	wantStates := []cashu.ProofState{cashu.PROOF_SPENT, cashu.PROOF_UNSPENT, cashu.PROOF_PENDING, cashu.PROOF_SPENT}
	if len(got) != len(wantStates) {
		t.Fatalf("got %d states, want %d", len(got), len(wantStates))
	}
	for i, want := range wantStates {
		if got[i].State != want || got[i].Y.ToHex() != []string{y1.ToHex(), y3.ToHex(), y2.ToHex(), y1.ToHex()}[i] {
			t.Errorf("state %d = (%s, %s), want (%s, %s)", i, got[i].Y, got[i].State, []string{y1.ToHex(), y3.ToHex(), y2.ToHex(), y1.ToHex()}[i], want)
		}
	}
	if got[0].Witness == nil || *got[0].Witness != witness || got[1].Witness != nil || got[2].Witness != nil {
		t.Fatalf("unexpected witnesses: %+v", got)
	}
}

func TestCheckProofStateRejectsNilY(t *testing.T) {
	db := &checkProofStateDB{}
	m := Mint{MintDB: db} //nolint:exhaustruct
	if _, err := CheckProofState(context.Background(), &m, []cashu.WrappedPublicKey{{}}); err == nil {
		t.Fatal("CheckProofState() error = nil, want nil-Y error")
	}
	if db.gotYs != nil {
		t.Fatal("database queried for invalid Y")
	}
}
