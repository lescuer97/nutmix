package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
)

func ParseVerifyProofError(proofError error) (cashu.ErrorCode, *string) {
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
	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.UNKNOWN, &message
	}

	return cashu.TOKEN_NOT_VERIFIED, nil

}

func GetChangeOutput(overpaidFees uint64, outputs []cashu.BlindedMessage) []cashu.BlindedMessage {
	amounts := cashu.AmountSplit(overpaidFees)
	// if there are more outputs then amount to change.
	// we size down the total amount of blind messages
	switch {
	case len(amounts) > len(outputs):
		for i := range outputs {
			outputs[i].Amount = amounts[i]
		}

	default:
		outputs = outputs[:len(amounts)]

		for i := range outputs {
			outputs[i].Amount = amounts[i]
		}

	}
	return outputs
}

// Sets some values being used by the mint like seen, secretY, seen, and pending state
func GetAndCalculateProofsValues(proofs *cashu.Proofs) (uint64, []string, error) {
	now := time.Now().Unix()
	var totalAmount uint64
	var SecretsList []string
	for i, proof := range *proofs {
		totalAmount += proof.Amount
		SecretsList = append(SecretsList, proof.Secret)

		p, err := proof.HashSecretToCurve()

		if err != nil {
			return 0, SecretsList, fmt.Errorf("proof.HashSecretToCurve(). %w", err)
		}
		(*proofs)[i] = p
		(*proofs)[i].SeenAt = now
	}

	return totalAmount, SecretsList, nil
}
