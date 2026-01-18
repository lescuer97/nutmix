package cashu

import (
	"fmt"
	"strconv"
)

type PostSwapRequest struct {
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

func (p *PostSwapRequest) ValidateSigflag() error {
	sigAllCheck, err := checkForSigAll(p.Inputs)
	if err != nil {
		return fmt.Errorf("checkForSigAll(p.Inputs). %w", err)
	}
	if sigAllCheck.sigFlag == SigAll {
		firstProof := p.Inputs[0]
		firstSpendCondition, err := firstProof.parseSpendCondition()
		if err != nil {
			return fmt.Errorf("p.Inputs[0].parseSpendCondition(). %w", err)
		}
		firstWitness, err := firstProof.parseWitness()
		if err != nil {
			return fmt.Errorf("p.Inputs[0].parseWitness(). %w", err)
		}

		if firstSpendCondition == nil || firstWitness == nil {
			return ErrInvalidSpendCondition
		}

		if firstWitness.Signatures == nil {
			return ErrNoValidSignatures
		}

		// check the conditions are met
		err = p.verifyConditions()
		if err != nil {
			return fmt.Errorf("p.verifyConditions(). %w", err)
		}

		// makes message
		msg := p.makeSigAllMsg()

		signatures, err := checkValidSignature(msg, sigAllCheck.pubkeys, firstWitness.Signatures)
		if err != nil {
			return fmt.Errorf("checkValidSignature(msg, pubkeys, firstWitness.Signatures). %w", err)
		}
		if signatures >= sigAllCheck.signaturesRequired {
			return nil
		}

		if firstProof.timelockPassed(firstSpendCondition) {
			signatures, err := checkValidSignature(msg, sigAllCheck.refundPubkeys, firstWitness.Signatures)
			if err != nil {
				return fmt.Errorf("checkValidSignature(msg, refundPubkeys, firstWitness.Signatures). %w", err)
			}
			if signatures >= sigAllCheck.signaturesRequiredRefund {
				return nil
			}
		}

		return ErrNotEnoughSignatures
	}
	return nil
}

func (p *PostSwapRequest) verifyConditions() error {
	firstProof := p.Inputs[0]
	firstSpendCondition, err := firstProof.parseSpendCondition()
	if err != nil {
		return nil
	}

	for _, proof := range p.Inputs {
		spendCondition, err := proof.parseSpendCondition()
		if err != nil {
			return nil
		}

		if spendCondition.Data.Data != firstSpendCondition.Data.Data {
			return fmt.Errorf("not same data field %w", ErrInvalidSpendCondition)
		}

		if spendCondition.Data.Tags.originalTag != firstSpendCondition.Data.Tags.originalTag {
			return fmt.Errorf("not same tags %w", ErrInvalidSpendCondition)
		}

	}
	return nil
}

// makeSigAllMsg creates the message for SIG_ALL signature verification
// Format: secret_0 || C_0 || ... || secret_n || C_n || amount_0 || B_0 || ... || amount_m || B_m
func (p *PostSwapRequest) makeSigAllMsg() string {
	message := ""
	for _, proof := range p.Inputs {
		message = message + proof.Secret + proof.C.String()
	}
	for _, blindMessage := range p.Outputs {
		message = message + strconv.FormatUint(blindMessage.Amount, 10) + blindMessage.B_.String()
	}
	return message
}

type PostSwapResponse struct {
	Signatures []BlindSignature `json:"signatures"`
}
