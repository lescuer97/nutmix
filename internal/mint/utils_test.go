package mint

import (
	"context"
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
)

const RegtestRequest string = "lnbcrt10u1pnxrpvhpp535rl7p9ze2dpgn9mm0tljyxsm980quy8kz2eydj7p4awra453u9qdqqcqzzsxqyz5vqsp55mdr2l90rhluaz9v3cmrt0qgjusy2dxsempmees6spapqjuj9m5q9qyyssq863hqzs6lcptdt7z5w82m4lg09l2d27al2wtlade6n4xu05u0gaxfjxspns84a73tl04u3t0pv4lveya8j0eaf9w7y5pstu70grpxtcqla7sxq"

func TestIsInternalTransactionSuccess(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	defer mint.MintDB.Rollback(ctx, tx)

	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v ", err)
	}

	mintRequest := cashu.MintRequestDB{
		Quote:       "quote1",
		Request:     RegtestRequest,
		RequestPaid: false,
		State:       cashu.UNPAID,
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
	defer mint.MintDB.Rollback(ctx, tx)

	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v ", err)
	}

	mintRequest := cashu.MintRequestDB{
		Quote:       "quote1",
		Request:     "wrong request",
		RequestPaid: false,
		State:       cashu.UNPAID,
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
