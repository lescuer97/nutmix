package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
)

func ParseErrorToCashuErrorCode(proofError error) (cashu.ErrorCode, *string) {
	switch {
	case errors.Is(proofError, cashu.ErrEmptyWitness):

		message := "Empty Witness"
		return cashu.UNKNOWN, &message
	case errors.Is(proofError, cashu.ErrNoValidSignatures):
		return cashu.TOKEN_NOT_VERIFIED, nil
	case errors.Is(proofError, cashu.ErrNotEnoughSignatures):
		return cashu.TOKEN_NOT_VERIFIED, nil
	case errors.Is(proofError, cashu.ErrLocktimePassed):
		message := cashu.ErrLocktimePassed.Error()
		return cashu.UNKNOWN, &message
	case errors.Is(proofError, cashu.UsingInactiveKeyset):
		return cashu.INACTIVE_KEYSET, nil
	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.UNKNOWN, &message
	}

	return cashu.TOKEN_NOT_VERIFIED, nil

}

// Sets some values being used by the mint like seen, secretY, seen, and pending state
func GetAndCalculateProofsValues(proofs *cashu.Proofs) (uint64, []string, error) {
	now := time.Now().Unix()
	var totalAmount uint64
	var SecretsList []string
	for i, proof := range *proofs {
		totalAmount += proof.Amount

		p, err := proof.HashSecretToCurve()

		if err != nil {
			return 0, SecretsList, fmt.Errorf("proof.HashSecretToCurve(). %w", err)
		}
		SecretsList = append(SecretsList, p.Y)
		(*proofs)[i] = p
		(*proofs)[i].SeenAt = now
	}

	return totalAmount, SecretsList, nil
}
