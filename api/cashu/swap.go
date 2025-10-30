package cashu

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type PostSwapRequest struct {
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

func (p *PostSwapRequest) ValidateSigflag() error {
	sigFlagValidation, err := checkForSigAll(p.Inputs)
	if err != nil {
		return fmt.Errorf("checkForSigAll(p.Inputs). %w", err)
	}
	if sigFlagValidation.sigFlag == SigAll {

		firstSpendCondition, err := p.Inputs[0].parseSpendCondition()
		if err != nil {
			return fmt.Errorf("p.Inputs[0].parseWitnessAndSecret(). %w", err)
		}
		firstWitness, err := p.Inputs[0].parseWitness()
		if err != nil {
			return fmt.Errorf("p.Inputs[0].parseWitnessAndSecret(). %w", err)
		}
		if firstSpendCondition == nil || firstWitness == nil {
			return ErrInvalidSpendCondition
		}

		if firstWitness.Signatures == nil {
			return ErrNoValidSignatures
		}

		// check tha conditions are met
		err = p.verifyConditions()
		if err != nil {
			return fmt.Errorf("p.verifyConditions(). %w", err)
		}

		// makes message
		msg := p.makeSigAllMsg()

		pubkeys, err := p.Inputs[0].Pubkeys()
		if err != nil {
			return fmt.Errorf("p.Inputs[0].Pubkeys(). %w", err)
		}

		amountOfSigs, err := checkValidSignature(msg, pubkeys, firstWitness.Signatures)
		if err != nil {
			return err
		}

		if amountOfSigs >= sigFlagValidation.signaturesRequired {
			return nil
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

		if string(spendCondition.Data.Tags.originalTag) != string(firstSpendCondition.Data.Tags.originalTag) {
			return fmt.Errorf("not same tags %w", ErrInvalidSpendCondition)
		}

	}
	return nil
}

func (p *PostSwapRequest) firstProofValues() error {
	firstProof := p.Inputs[0]
	firstSpendCondition, err := firstProof.parseSpendCondition()
	if err != nil {
		return nil
	}

	firstTagString, err := json.Marshal(firstSpendCondition.Data.Tags)
	if err != nil {
		return nil
	}

	for _, proof := range p.Inputs {
		spendCondition, err := proof.parseSpendCondition()
		if err != nil {
			return nil
		}

		if spendCondition.Data.Data != firstSpendCondition.Data.Data {
			return ErrInvalidSpendCondition
		}
		if spendCondition.Data.Tags.NSigRefund != firstSpendCondition.Data.Tags.NSigRefund {
			return ErrInvalidSpendCondition
		}

		tagString, err := json.Marshal(spendCondition.Data.Tags)
		if err != nil {
			return nil
		}

		if string(tagString) != string(firstTagString) {
			return ErrInvalidSpendCondition
		}

	}
	return nil
}

func (p *PostSwapRequest) makeSigAllMsg() string {
	var msg strings.Builder
	for _, proof := range p.Inputs {
		msg.WriteString(proof.Secret)
	}
	for _, blindMessage := range p.Outputs {
		B_Hex := hex.EncodeToString(blindMessage.B_.SerializeCompressed())
		msg.WriteString(B_Hex)
	}
	return msg.String()
}

type PostSwapResponse struct {
	Signatures []BlindSignature `json:"signatures"`
}
