package mint

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

const RegtestRequest string = "lnbcrt10u1pnxrpvhpp535rl7p9ze2dpgn9mm0tljyxsm980quy8kz2eydj7p4awra453u9qdqqcqzzsxqyz5vqsp55mdr2l90rhluaz9v3cmrt0qgjusy2dxsempmees6spapqjuj9m5q9qyyssq863hqzs6lcptdt7z5w82m4lg09l2d27al2wtlade6n4xu05u0gaxfjxspns84a73tl04u3t0pv4lveya8j0eaf9w7y5pstu70grpxtcqla7sxq"

func TestIsInternalTransactionSuccess(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v ", err)
	}

	mintRequest := cashu.MintRequestDB{
		Quote:   "quote1",
		Request: RegtestRequest,
		State:   cashu.UNPAID,
	}
	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx,mintRequest ): %+v ", err)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}
	isInternal, err := mint.IsInternalTransaction(RegtestRequest)
	if err != nil {
		t.Fatalf("mint.IsInternalTransaction(RegtestRequest): %+v ", err)
	}

	if !isInternal {
		t.Error("should be internal transaction")
	}

}
func TestIsInternalTransactionFail(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v ", err)
	}

	mintRequest := cashu.MintRequestDB{
		Quote:   "quote1",
		Request: "wrong request",
		State:   cashu.UNPAID,
	}
	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx,mintRequest ): %+v ", err)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}
	isInternal, err := mint.IsInternalTransaction(RegtestRequest)
	if err != nil {
		t.Fatalf("mint.IsInternalTransaction(RegtestRequest): %+v ", err)
	}

	if isInternal {
		t.Error("should be external transaction")
	}

}

func TestVerifyUnitOfProofFail(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	err := mint.Signer.RotateKeyset(cashu.EUR, 0, 240)
	if err != nil {
		t.Fatalf("mint.Signer.RotateKeyset(cashu.EUR, 0): %+v ", err)
	}

	keysets, err := mint.Signer.GetKeysets()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeys(): %+v ", err)
	}
	proofs := cashu.Proofs{cashu.Proof{Id: "00bfa73302d12ffd"}, cashu.Proof{Id: "00bfa73302d12ffd"}, cashu.Proof{Id: "0061287798d19b10"}}

	_, err = mint.CheckProofsAreSameUnit(proofs, keysets.Keysets)
	if err == nil {
		t.Errorf("should have failed because of there are different units: %+v ", err)
	}
	if !errors.Is(err, cashu.ErrNotSameUnits) {
		t.Errorf("Error should be Not Same units. %v", err)
	}
}
func TestVerifyUnitOfProofPass(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	err := mint.Signer.RotateKeyset(cashu.EUR, 0, 240)
	if err != nil {
		t.Fatalf("mint.Signer.RotateKeyset(cashu.EUR, 0): %+v ", err)
	}

	keysets, err := mint.Signer.GetKeysets()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeys(): %+v ", err)
	}
	proofs := cashu.Proofs{cashu.Proof{Id: "00bfa73302d12ffd"}, cashu.Proof{Id: "00bfa73302d12ffd"}, cashu.Proof{Id: "00bfa73302d12ffd"}}

	unit, err := mint.CheckProofsAreSameUnit(proofs, keysets.Keysets)
	if err != nil {
		t.Errorf("There should not be and error. %v", err)
	}
	if unit != cashu.Sat {
		t.Errorf("Unit should be Sat. %v", err)
	}

}

func TestVerifyOutputsFailRepeatedOutput(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	err := mint.Signer.RotateKeyset(cashu.EUR, 0, 240)
	if err != nil {
		t.Fatalf("mint.Signer.RotateKeyset(cashu.EUR, 0): %+v ", err)
	}

	keysets, err := mint.Signer.GetKeysets()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeys(): %+v ", err)
	}
	b_bytes1, err := hex.DecodeString("02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2")
	if err != nil {
		t.Fatalf("Error decoding b_bytes1: %+v", err)
	}
	B_1, err := secp256k1.ParsePubKey(b_bytes1)
	if err != nil {
		t.Fatalf("Error parsing B_1: %+v", err)
	}
	b_bytes2, err := hex.DecodeString("02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107bb2")
	if err != nil {
		t.Fatalf("Error decoding b_bytes2: %+v", err)
	}
	B_2, err := secp256k1.ParsePubKey(b_bytes2)
	if err != nil {
		t.Fatalf("Error parsing B_2: %+v", err)
	}
	outputs := []cashu.BlindedMessage{{Id: "0198c6516691814bf519f1736b124b28406dc954d1406c4ace4610c42865b55239", B_: cashu.WrappedPublicKey{PublicKey: B_1}}, {Id: "0198c6516691814bf519f1736b124b28406dc954d1406c4ace4610c42865b55239", B_: cashu.WrappedPublicKey{PublicKey: B_2}}, {Id: "0198c6516691814bf519f1736b124b28406dc954d1406c4ace4610c42865b55239", B_: cashu.WrappedPublicKey{PublicKey: B_2}}}

	tx, err := mint.MintDB.GetTx(context.Background())
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)

	}
	_, err = mint.VerifyOutputs(tx, outputs, keysets.Keysets)
	if err == nil {
		t.Errorf("should have failed because of there are repeated outputs: %+v ", err)
	}
	if !errors.Is(err, cashu.ErrRepeatedOutput) {
		t.Errorf("Error there should be a repeated output. %v", err)
	}
	err = tx.Commit(context.Background())
	if err != nil {
		t.Fatalf("Could not commit   tx: %+v ", err)
	}
}
