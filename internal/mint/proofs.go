package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func CheckProofState(ctx context.Context, mint *Mint, Ys []cashu.WrappedPublicKey) ([]cashu.CheckState, error) {
	for _, y := range Ys {
		if y.PublicKey == nil {
			return nil, fmt.Errorf("proof state Y cannot be nil")
		}
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		err := mint.MintDB.Rollback(ctx, tx)
		if err != nil {
			if !errors.Is(err, pgx.ErrTxClosed) {
				slog.Warn("rotate keyset sql transaction error", slog.Any("error", err))
			}
		}
	}()

	// set as unspent
	proofs, err := mint.MintDB.GetProofsFromSecretCurve(tx, Ys)
	if err != nil {
		return nil, fmt.Errorf("database.CheckListOfProofsBySecretCurve(pool, Ys). %w", err)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	proofsByY := make(map[string]cashu.Proof, len(proofs))
	for _, proof := range proofs {
		proofsByY[proof.Y.ToHex()] = proof
	}

	var states = make([]cashu.CheckState, len(Ys))
	for i, state := range Ys {
		checkState := cashu.CheckState{
			Y:       state,
			State:   cashu.PROOF_UNSPENT,
			Witness: nil,
		}

		if proof, ok := proofsByY[state.ToHex()]; ok {
			checkState.State = proof.State
			if proof.Witness != "" {
				witness := proof.Witness
				checkState.Witness = &witness
			}
		}

		states[i] = checkState
	}

	return states, nil
}
