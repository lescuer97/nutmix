package cashu

import (
	"fmt"
	"strings"
)

type ACTION_STATE string

const (
	UNPAID  ACTION_STATE = "UNPAID"
	PAID    ACTION_STATE = "PAID"
	PENDING ACTION_STATE = "PENDING"
	ISSUED  ACTION_STATE = "ISSUED"
)

type MeltRequestDB struct {
	Quote      string `json:"quote"`
	Unit       string `json:"unit"`
	Expiry     int64  `json:"expiry"`
	Amount     uint64 `json:"amount"`
	FeeReserve uint64 `json:"fee_reserve" db:"fee_reserve"`
	FeePaid    uint64 `json:"paid_fee" db:"fee_paid"`
	// Deprecated: Should be removed after all main wallets change to the new State format
	RequestPaid     bool         `json:"paid" db:"request_paid"`
	Request         string       `json:"request"`
	Melted          bool         `json:"melted"`
	State           ACTION_STATE `json:"state"`
	PaymentPreimage string       `json:"payment_preimage"`
	SeenAt          int64        `json:"seen_at"`
	Mpp             bool         `json:"mpp"`
	CheckingId      string       `json:"checking_id"`
}

func (meltRequest *MeltRequestDB) GetPostMeltQuoteResponse() PostMeltQuoteBolt11Response {
	return PostMeltQuoteBolt11Response{
		Quote:           meltRequest.Quote,
		Amount:          meltRequest.Amount,
		FeeReserve:      meltRequest.FeeReserve,
		Paid:            meltRequest.RequestPaid,
		Expiry:          meltRequest.Expiry,
		State:           meltRequest.State,
		PaymentPreimage: meltRequest.PaymentPreimage,
		Request:         meltRequest.Request,
		Unit:            meltRequest.Unit,
	}

}

type PostMeltQuoteBolt11Options struct {
	Mpp map[string]uint64 `json:"mpp"`
}

type PostMeltQuoteBolt11Request struct {
	Request string                     `json:"request"`
	Unit    string                     `json:"unit"`
	Options PostMeltQuoteBolt11Options `json:"options"`
}

func (p PostMeltQuoteBolt11Request) IsMpp() uint64 {
	if p.Options.Mpp["amount"] != 0 {
		return p.Options.Mpp["amount"]
	}
	return 0
}

type PostMeltQuoteBolt11Response struct {
	Quote      string `json:"quote"`
	Amount     uint64 `json:"amount"`
	FeeReserve uint64 `json:"fee_reserve"`
	// Deprecated: Should be removed after all main wallets change to the new State format
	Paid            bool             `json:"paid"`
	Expiry          int64            `json:"expiry"`
	State           ACTION_STATE     `json:"state"`
	Change          []BlindSignature `json:"change"`
	Unit            string           `json:"unit"`
	Request         string           `json:"request"`
	PaymentPreimage string           `json:"payment_preimage"`
}

type PostMeltBolt11Request struct {
	Quote   string           `json:"quote"`
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

func (p *PostMeltBolt11Request) ValidateSigflag() error {
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

		// check the conditions are met
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

func (p *PostMeltBolt11Request) verifyConditions() error {
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

func (p *PostMeltBolt11Request) makeSigAllMsg() string {
	var msg strings.Builder
	for _, proof := range p.Inputs {
		msg.WriteString(proof.Secret)
	}
	for _, blindMessage := range p.Outputs {
		msg.WriteString(blindMessage.B_.String())
	}
	msg.WriteString(p.Quote)
	return msg.String()
}
