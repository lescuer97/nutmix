package mint

import (
	"context"
	"fmt"

	"github.com/lescuer97/nutmix/api/cashu"
)

func (m *Mint) Restore(ctx context.Context, request cashu.PostRestoreRequest) (cashu.PostRestoreResponse, error) {
	blindingFactors := make([]cashu.WrappedPublicKey, len(request.Outputs))

	for i, output := range request.Outputs {
		blindingFactors[i] = output.B_
	}

	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostRestoreResponse{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	blindRecoverySigs, err := m.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors)
	if err != nil {
		return cashu.PostRestoreResponse{}, fmt.Errorf("m.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors) %w", err)
	}
	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return cashu.PostRestoreResponse{}, fmt.Errorf("m.MintDB.Commit(ctx, tx) %w", err)
	}

	restoredBlindSigs := make([]cashu.BlindSignature, len(blindRecoverySigs))
	restoredBlindMessage := make([]cashu.BlindedMessage, len(blindRecoverySigs))

	for i, sigRecover := range blindRecoverySigs {
		restoredSig, restoredMessage := sigRecover.GetSigAndMessage()
		restoredBlindSigs[i] = restoredSig
		restoredBlindMessage[i] = restoredMessage
	}
	return cashu.PostRestoreResponse{Signatures: restoredBlindSigs, Promises: restoredBlindSigs, Outputs: restoredBlindMessage}, nil
}
