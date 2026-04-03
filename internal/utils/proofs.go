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
		return cashu.OUTPUTS_ALREADY_SIGNED, &message

	case errors.Is(proofError, cashu.ErrEmptyWitness):

		message := "Empty Witness"
		return cashu.UNKNOWN, &message

	case errors.Is(proofError, cashu.ErrPaymentNoRoute):
		message := "No route found for payment"
		return cashu.LIGHTNING_PAYMENT_FAILED, &message
	case errors.Is(proofError, cashu.ErrNoValidSignatures):
		return cashu.PROOF_VERIFICATION_FAILED, nil
	case errors.Is(proofError, cashu.ErrNotEnoughSignatures):
		return cashu.PROOF_VERIFICATION_FAILED, nil
	case errors.Is(proofError, cashu.ErrInvalidProof):
		message := cashu.ErrInvalidProof.Error()
		return cashu.PROOF_VERIFICATION_FAILED, &message
	case errors.Is(proofError, cashu.ErrInvalidBlindMessage):
		message := cashu.ErrInvalidBlindMessage.Error()
		return cashu.PROOF_VERIFICATION_FAILED, &message
	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.PROOF_VERIFICATION_FAILED, &message

	case errors.Is(proofError, cashu.ErrAmountlessInvoiceNotSupported):
		message := cashu.ErrAmountlessInvoiceNotSupported.Error()
		return cashu.AMOUNT_LESS_INVOICE_NOT_SUPPORTED, &message

	case errors.Is(proofError, cashu.ErrAmountOutsideLimit):
		message := cashu.ErrAmountOutsideLimit.Error()
		return cashu.INSUFICIENT_OUTSIDE_LIMIT, &message

	case errors.Is(proofError, cashu.ErrMintintDisabled):
		message := cashu.ErrMintintDisabled.Error()
		return cashu.MINTING_DISABLED, &message

	case errors.Is(proofError, cashu.ErrMintRequestAlreadyIssued):
		message := cashu.ErrMintRequestAlreadyIssued.Error()
		return cashu.QUOTE_ALREADY_ISSUED, &message

	case errors.Is(proofError, cashu.ErrLocktimePassed):
		message := cashu.ErrLocktimePassed.Error()
		return cashu.UNKNOWN, &message
	case errors.Is(proofError, cashu.ErrUsingInactiveKeyset):
		return cashu.INACTIVE_KEYSET, nil
	case errors.Is(proofError, cashu.ErrMeltAlreadyPaid):
		message := cashu.ErrMeltAlreadyPaid.Error()
		return cashu.INVOICE_ALREADY_PAID, &message

	case errors.Is(proofError, cashu.ErrProofSpent):
		message := cashu.ErrProofSpent.Error()
		return cashu.PROOF_ALREADY_SPENT, &message
	case errors.Is(proofError, cashu.ErrProofPending):
		message := cashu.ErrProofPending.Error()
		return cashu.PROOFS_PENDING, &message

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

	case errors.Is(proofError, cashu.ErrAmountNotEqualToInvoice):
		message := cashu.ErrAmountNotEqualToInvoice.Error()
		return cashu.AMOUNT_NOT_EQUAL_TO_INVOICE, &message

	case errors.Is(proofError, cashu.ErrRequestNotPaid):
		message := cashu.ErrRequestNotPaid.Error()
		return cashu.REQUEST_NOT_PAID, &message

	case errors.Is(proofError, cashu.ErrQuoteIsPending):
		message := cashu.ErrQuoteIsPending.Error()
		return cashu.QUOTE_PENDING, &message

	case errors.Is(proofError, cashu.ErrUnitNotSupported):
		message := cashu.ErrUnitNotSupported.Error()
		return cashu.UNIT_NOT_SUPPORTED, &message

	case errors.Is(proofError, cashu.ErrDifferentInputOutputUnit):
		message := cashu.ErrDifferentInputOutputUnit.Error()
		return cashu.INPUT_OUTPUT_NOT_SAME_UNIT, &message

	case errors.Is(proofError, cashu.ErrInvalidPreimage):
		message := cashu.ErrInvalidPreimage.Error()
		return cashu.PROOF_VERIFICATION_FAILED, &message

	case errors.Is(proofError, cashu.ErrBlindMessageAlreadySigned):
		message := cashu.ErrBlindMessageAlreadySigned.Error()
		return cashu.OUTPUTS_ALREADY_SIGNED, &message

	case strings.Contains(proofError.Error(), "could not obtain lock"):
		message := "Transaction is already pending"
		return cashu.QUOTE_PENDING, &message

	case errors.Is(proofError, cashu.ErrPaymentFailed):
		message := cashu.ErrNotEnoughtProofs.Error()
		return cashu.LIGHTNING_PAYMENT_FAILED, &message

	case errors.Is(proofError, cashu.ErrMintQuoteNoPublicKey):
		return cashu.MINT_QUOTE_INVALID_PUB_KEY, nil

	case errors.Is(proofError, cashu.ErrMintQuoteNoValidSignature):
		return cashu.MINT_QUOTE_INVALID_SIG, nil

	case errors.Is(proofError, cashu.ErrCouldNotParsePublicKey):
		message := cashu.ErrCouldNotParsePublicKey.Error()
		return cashu.PROOF_VERIFICATION_FAILED, &message
	}

	return cashu.UNKNOWN, nil
}

// Sets the y and seen at field in by references and returns the Y's array
func GetAndCalculateProofsValues(proofs *cashu.Proofs) ([]cashu.WrappedPublicKey, error) {
	now := time.Now().Unix()
	var totalAmount uint64
	secretsList := make([]cashu.WrappedPublicKey, len(*proofs))
	for i, proof := range *proofs {
		totalAmount += proof.Amount

		p, err := proof.HashSecretToCurve()

		if err != nil {
			return nil, fmt.Errorf("proof.HashSecretToCurve(). %w", err)
		}
		secretsList[i] = p.Y
		(*proofs)[i] = p
		(*proofs)[i].SeenAt = now
	}

	return secretsList, nil
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
