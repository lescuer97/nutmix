package mint

import (
	"context"
	"fmt"
	"slices"

	"github.com/lescuer97/nutmix/api/cashu"
)

func CheckProofState(mint *Mint, Ys []string) ([]cashu.CheckState, error) {
	var states []cashu.CheckState
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return states, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer tx.Rollback(ctx)
	// set as unspent
	proofs, err := mint.MintDB.GetProofsFromSecretCurve(tx, Ys)
	if err != nil {
		return states, fmt.Errorf("database.CheckListOfProofsBySecretCurve(pool, Ys). %w", err)
	}

	proofsForRemoval := make([]cashu.Proof, 0)

	for _, state := range Ys {

		pendingAndSpent := false

		checkState := cashu.CheckState{
			Y:       state,
			State:   cashu.PROOF_UNSPENT,
			Witness: nil,
		}

		switch {
		// Check if is in list of spents and if its also pending add it for removal of pending list
		case slices.ContainsFunc(proofs, func(p cashu.Proof) bool {
			compare := p.Y == state
			if p.Witness != "" {
				checkState.Witness = &p.Witness
			}
			if compare && pendingAndSpent {

				proofsForRemoval = append(proofsForRemoval, p)
			}
			return compare
		}):
			checkState.State = cashu.PROOF_SPENT
		}

		states = append(states, checkState)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return states, fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}
	return states, nil
}
