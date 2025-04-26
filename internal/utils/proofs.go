package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
)

func ParseErrorToCashuErrorCode(proofError error) (cashu.ErrorCode, *string) {
	switch {
	case errors.Is(proofError, cashu.ErrBlindMessageAlreadySigned):
		message := cashu.ErrBlindMessageAlreadySigned.Error()
		return cashu.OUTPUT_BLINDED_MESSAGE_ALREADY_SIGNED, &message

	case errors.Is(proofError, cashu.ErrEmptyWitness):

		message := "Empty Witness"
		return cashu.UNKNOWN, &message
	case errors.Is(proofError, cashu.ErrNoValidSignatures):
		return cashu.TOKEN_NOT_VERIFIED, nil
	case errors.Is(proofError, cashu.ErrNotEnoughSignatures):
		return cashu.TOKEN_NOT_VERIFIED, nil
	case errors.Is(proofError, cashu.ErrInvalidProof):
		message := cashu.ErrInvalidProof.Error()
		return cashu.TOKEN_NOT_VERIFIED, &message
	case errors.Is(proofError, cashu.ErrInvalidBlindMessage):
		message := cashu.ErrInvalidBlindMessage.Error()
		return cashu.TOKEN_NOT_VERIFIED, &message
	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.TOKEN_NOT_VERIFIED, &message

	case errors.Is(proofError, cashu.ErrLocktimePassed):
		message := cashu.ErrLocktimePassed.Error()
		return cashu.UNKNOWN, &message
	case errors.Is(proofError, cashu.UsingInactiveKeyset):
		return cashu.INACTIVE_KEYSET, nil
	case errors.Is(proofError, cashu.ErrMeltAlreadyPaid):
		message := cashu.ErrMeltAlreadyPaid.Error()
		return cashu.INVOICE_ALREADY_PAID, &message

	case errors.Is(proofError, cashu.ErrProofSpent):
		message := cashu.ErrProofSpent.Error()
		return cashu.TOKEN_ALREADY_SPENT, &message

	case errors.Is(proofError, cashu.ErrNotSameUnits):
		message := cashu.ErrNotSameUnits.Error()
		return cashu.TRANSACTION_NOT_BALANCED, &message
	case errors.Is(proofError, cashu.ErrNotEnoughtProofs):
		message := cashu.ErrNotEnoughtProofs.Error()
		return cashu.TRANSACTION_NOT_BALANCED, &message
	case errors.Is(proofError, cashu.ErrUnbalanced):
		message := cashu.ErrUnbalanced.Error()
		return cashu.TRANSACTION_NOT_BALANCED, &message
	case errors.Is(proofError, cashu.ErrRepeatedOutput):
		message := cashu.ErrRepeatedOutput.Error()
		return cashu.DUPLICATE_OUTPUTS, &message
	case errors.Is(proofError, cashu.ErrRepeatedInput):
		message := cashu.ErrRepeatedInput.Error()
		return cashu.DUPLICATE_INPUTS, &message

	case errors.Is(proofError, cashu.ErrUnitNotSupported):
		message := cashu.ErrUnitNotSupported.Error()
		return cashu.UNIT_NOT_SUPPORTED, &message

	case errors.Is(proofError, cashu.ErrDifferentInputOutputUnit):
		message := cashu.ErrDifferentInputOutputUnit.Error()
		return cashu.INPUT_OUTPUT_NOT_SAME_UNIT, &message

	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.TOKEN_NOT_VERIFIED, &message

	case errors.Is(proofError, cashu.ErrBlindMessageAlreadySigned):
		message := cashu.ErrBlindMessageAlreadySigned.Error()
		return cashu.OUTPUT_BLINDED_MESSAGE_ALREADY_SIGNED, &message

	case strings.Contains(proofError.Error(), "could not obtain lock"):
		message := "Transaction is already pending"
		return cashu.QUOTE_PENDING, &message

	case errors.Is(proofError, cashu.ErrPaymentFailed):
		message := cashu.ErrNotEnoughtProofs.Error()
		return cashu.LIGHTNING_PAYMENT_FAILED, &message

	}

	return cashu.UNKNOWN, nil
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
func GetMessagesForChange(overpaidFees uint64, outputs []cashu.BlindedMessage) []cashu.BlindedMessage {
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
