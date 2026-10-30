//nolint:exhaustruct // Fixtures specify only fields relevant to the error path.
package mint

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestOutputAvailabilityErrorsAcrossReservationPaths(t *testing.T) {
	for _, signed := range []bool{false, true} {
		for _, path := range []string{"mint", "melt", "swap availability"} {
			name := path + " pending"
			want, code := cashu.ErrOutputPending, cashu.OUTPUTS_PENDING
			if signed {
				name, want, code = path+" signed", cashu.ErrBlindMessageAlreadySigned, cashu.OUTPUTS_ALREADY_SIGNED
			}
			t.Run(name, func(t *testing.T) {
				key := cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()}
				outputs := cashu.BlindedMessages{{B_: key, Id: "keyset", Amount: 1}}
				db := &mockdb.MockDB{
					MintRequest: []cashu.MintRequestDB{{Quote: "mint", State: cashu.PAID}},
					MeltRequest: []cashu.MeltRequestDB{{Quote: "melt", State: cashu.UNPAID}, {Quote: "owner", State: cashu.PENDING}},
				}
				if signed {
					db.RecoverSigDB = []cashu.RecoverSigDB{{B_: key, C_: key, Id: "keyset", Amount: 1}}
				} else if err := db.SaveMeltChange(nil, outputs, "owner"); err != nil {
					t.Fatal(err)
				}
				mint := &Mint{MintDB: db}
				var err error
				switch path {
				case "mint":
					_, err = mint.reserveMintQuoteForIssuance(context.Background(), cashu.PostMintBolt11Request{Quote: "mint", Outputs: outputs})
				case "melt":
					_, err = mint.reserveMeltInputsAndMarkPending(context.Background(), cashu.PostMeltBolt11Request{
						Quote: "melt", Outputs: outputs, Inputs: cashu.Proofs{{Secret: "unspent", Id: "keyset", Amount: 1}},
					})
				case "swap availability":
					err = mint.ValidateOutputsNotSpent(nil, outputs)
				}
				if !errors.Is(err, want) || errors.Is(err, cashu.ErrQuoteIsPending) {
					t.Fatalf("wrong availability error: %v", err)
				}
				got, _ := utils.ParseErrorToCashuErrorCode(fmt.Errorf("operation: %w", err))
				if got != code {
					t.Fatalf("error code=%d want=%d", got, code)
				}
			})
		}
	}
}

func TestPendingQuotesRemainDistinctFromPendingOutputs(t *testing.T) {
	db := &mockdb.MockDB{
		MintRequest: []cashu.MintRequestDB{{Quote: "mint", State: cashu.PENDING}},
		MeltRequest: []cashu.MeltRequestDB{{Quote: "melt", State: cashu.PENDING}},
	}
	mint := &Mint{MintDB: db}
	_, mintErr := mint.reserveMintQuoteForIssuance(context.Background(), cashu.PostMintBolt11Request{Quote: "mint"})
	_, meltErr := mint.reserveMeltInputsAndMarkPending(context.Background(), cashu.PostMeltBolt11Request{Quote: "melt"})
	for _, err := range []error{mintErr, meltErr} {
		code, _ := utils.ParseErrorToCashuErrorCode(err)
		if !errors.Is(err, cashu.ErrQuoteIsPending) || errors.Is(err, cashu.ErrOutputPending) || code != cashu.QUOTE_PENDING {
			t.Fatalf("quote state misclassified: %v code=%d", err, code)
		}
	}
}
