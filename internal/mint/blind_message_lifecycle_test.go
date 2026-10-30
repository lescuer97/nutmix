//nolint:exhaustruct // Fixtures specify only fields relevant to the lifecycle scenario.
package mint

import (
	"context"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/signer"
)

type lifecycleSigner struct{ signer.Signer }

func (lifecycleSigner) SignBlindMessages(outputs []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	signatures := make([]cashu.BlindSignature, len(outputs))
	recovery := make([]cashu.RecoverSigDB, len(outputs))
	for i, output := range outputs {
		signatures[i] = cashu.BlindSignature{C_: output.B_, Amount: output.Amount, Id: output.Id}
		recovery[i] = cashu.RecoverSigDB{B_: output.B_, C_: output.B_, Amount: output.Amount, Id: output.Id}
	}
	return signatures, recovery, nil
}

type lifecycleCommitDB struct {
	*mockdb.MockDB
	commitErr error
	committed bool
}

func (db *lifecycleCommitDB) Commit(context.Context, pgx.Tx) error {
	db.committed = true
	return db.commitErr
}

func TestMintLifecyclePersistsProvenanceBeforeReturningSignatures(t *testing.T) {
	commitFailure := errors.New("commit failed")
	for _, commitErr := range []error{nil, commitFailure} {
		name := "success"
		if commitErr != nil {
			name = "commit failure"
		}
		t.Run(name, func(t *testing.T) {
			quote := cashu.MintRequestDB{Quote: "mint-quote", State: cashu.PENDING}
			db := &lifecycleCommitDB{MockDB: &mockdb.MockDB{MintRequest: []cashu.MintRequestDB{quote}}, commitErr: commitErr}
			observer := newObserverForTest()
			mint := &Mint{MintDB: db, Signer: lifecycleSigner{}, Observer: &observer}
			key := cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()}
			request := cashu.PostMintBolt11Request{Quote: quote.Quote, Outputs: cashu.BlindedMessages{{B_: key, Id: "keyset", Amount: 1}}}
			sigs, err := mint.signMintOutputsAndMarkIssued(context.Background(), request, quote)
			if !errors.Is(err, commitErr) || !db.committed {
				t.Fatalf("commit=%v error=%v", db.committed, err)
			}
			if commitErr != nil && len(sigs) != 0 {
				t.Fatal("returned signatures after failed commit")
			}
			if commitErr == nil && len(sigs) != 1 {
				t.Fatal("successful issuance did not return signature")
			}
			if len(db.RecoverSigDB) != 1 || db.RecoverSigDB[0].MintQuote != quote.Quote {
				t.Fatal("mint provenance missing from persistence request")
			}
		})
	}
}
