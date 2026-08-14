package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
)

// 10.3: a Postgres row-lock conflict (SQLSTATE 55P03) on the swap path must
// map to PROOFS_PENDING (11002), not QUOTE_PENDING (20005).
func TestParseErrorLockConflictMapsToProofsPending(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "55P03"} //nolint:exhaustruct
	err := fmt.Errorf("m.MintDB.SetProofsPending(tx, proofs). %w", pgErr)

	code, _ := ParseErrorToCashuErrorCode(err)
	if code != cashu.PROOFS_PENDING {
		t.Errorf("expected PROOFS_PENDING (11002), got %v", uint(code))
	}
}

// 10.4: unit-mismatch sentinels map to the precise codes 11009/11010.
func TestParseErrorUnitMismatchCodes(t *testing.T) {
	code, _ := ParseErrorToCashuErrorCode(fmt.Errorf("wrap. %w", cashu.ErrMultipleUnits))
	if code != cashu.MULTIPLE_UNITS_OUTPUT_INPUT {
		t.Errorf("expected MULTIPLE_UNITS_OUTPUT_INPUT (11009), got %v", uint(code))
	}

	code, _ = ParseErrorToCashuErrorCode(fmt.Errorf("wrap. %w", cashu.ErrDifferentInputOutputUnit))
	if code != cashu.INPUT_OUTPUT_NOT_SAME_UNIT {
		t.Errorf("expected INPUT_OUTPUT_NOT_SAME_UNIT (11010), got %v", uint(code))
	}

	if !errors.Is(fmt.Errorf("wrap. %w", cashu.ErrMultipleUnits), cashu.ErrMultipleUnits) {
		t.Error("ErrMultipleUnits should survive wrapping")
	}
}

func setListofEmptyBlindMessages(amounts int) []cashu.BlindedMessage {
	var messages []cashu.BlindedMessage
	for i := 0; i < amounts; i++ {
		message := cashu.BlindedMessage{
			B_:      cashu.WrappedPublicKey{PublicKey: nil},
			Id:      "mockid",
			Witness: "",
			Amount:  0,
		}
		messages = append(messages, message)
	}

	return messages
}
func TestGetChangeWithEnoughBlindMessages(t *testing.T) {
	emptyBlindMessages := setListofEmptyBlindMessages(10)

	// create change for value of 2
	change := GetMessagesForChange(2, emptyBlindMessages)

	if len(change) != 1 {
		t.Errorf("Incorrect size for change slice %v, should be 1", len(change))
	}

	if change[0].Amount != 2 {
		t.Errorf("Incorrect amount for change slice %v, should be 2", change[0].Amount)
	}

	// create change for a 0 amount
	change = GetMessagesForChange(0, emptyBlindMessages)

	if len(change) != 0 {
		t.Errorf("Incorrect size for change slice %v, should be 0", len(change))
	}
}

func TestGetChangeWithOutEnoughBlindMessages(t *testing.T) {
	emptyBlindMessages := setListofEmptyBlindMessages(1)

	// create change for value of 2
	change := GetMessagesForChange(10, emptyBlindMessages)

	if len(change) != 1 {
		t.Errorf("Incorrect size for change slice %v, should be 1", len(change))
	}

	if change[0].Amount != 2 {
		t.Errorf("Incorrect amount for change slice %v, should be 2", change[0].Amount)
	}
}

func MakeListofMockProofs(amounts int) []cashu.Proof {
	var proofs []cashu.Proof
	for i := 0; i < amounts; i++ {
		proof := cashu.Proof{
			C:       cashu.WrappedPublicKey{PublicKey: nil},
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   nil,
			Id:      "mockid",
			Secret:  "",
			Witness: "",
			State:   "",
			Amount:  0,
			SeenAt:  0,
		}
		proofs = append(proofs, proof)
	}

	return proofs
}

func TestGetValuesFromProofs(t *testing.T) {
	listOfProofs := cashu.Proofs{
		{
			C:       cashu.WrappedPublicKey{PublicKey: nil},
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   nil,
			Id:      "mockid",
			Secret:  "mockSecret",
			Witness: "",
			State:   "",
			Amount:  2,
			SeenAt:  0,
		},
		{
			C:       cashu.WrappedPublicKey{PublicKey: nil},
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   nil,
			Id:      "mockid",
			Secret:  "mockSecret2",
			Witness: "",
			State:   "",
			Amount:  6,
			SeenAt:  0,
		},
	}

	secretsList, err := GetAndCalculateProofsValues(&listOfProofs)
	if err != nil {
		t.Fatal("GetAndCalculateProofsValues(&listOfProofs)")
	}

	amount, err := listOfProofs.Amount()
	if err != nil {
		t.Fatalf("listOfProofs.Amount(): %v", err)
	}
	if amount != 8 {
		t.Errorf("Incorrect total amount %v. Should be 8", amount)
	}

	if secretsList[0].ToHex() != "02aa4a2c024e41bd87e8c2758d5a7c2d81e09afe52f67fc8a69768bd73d515e28f" {
		t.Errorf("Should be mock secret %v", secretsList[0].ToHex())
	}
	if listOfProofs[0].Y.ToHex() != "02aa4a2c024e41bd87e8c2758d5a7c2d81e09afe52f67fc8a69768bd73d515e28f" {
		t.Errorf("Incorrect Y: %v. ", listOfProofs[0].Y)
	}
}
