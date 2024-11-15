package mint

import (
	"fmt"
	"github.com/lescuer97/nutmix/api/cashu"
	"slices"
)

func CheckProofState(mint *Mint, Ys []string) ([]cashu.CheckState, error) {
	var states []cashu.CheckState
	// set as unspent
	proofs, err := mint.MintDB.GetProofsFromSecretCurve(Ys)
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
		// check if is in list of pending proofs
		case slices.ContainsFunc(mint.PendingProofs, func(p cashu.Proof) bool {
			checkState.Witness = &p.Witness
			return p.Y == state
		}):
			pendingAndSpent = true
			checkState.State = cashu.PROOF_PENDING
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

	// remove proofs from pending proofs
	if len(proofsForRemoval) != 0 {
		newPendingProofs := []cashu.Proof{}
		for _, proof := range mint.PendingProofs {
			if !slices.Contains(proofsForRemoval, proof) {
				newPendingProofs = append(newPendingProofs, proof)
			}
		}
	}
	return states, nil

}
