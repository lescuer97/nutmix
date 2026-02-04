package cashu

import (
	"fmt"
	"strconv"
)

type ACTION_STATE string

const (
	UNPAID  ACTION_STATE = "UNPAID"
	PAID    ACTION_STATE = "PAID"
	PENDING ACTION_STATE = "PENDING"
	ISSUED  ACTION_STATE = "ISSUED"
)

type MeltRequestDB struct {
	PaymentPreimage string       `json:"payment_preimage"`
	Unit            string       `json:"unit"`
	Request         string       `json:"request"`
	State           ACTION_STATE `json:"state"`
	Quote           string       `json:"quote"`
	CheckingId      string       `json:"checking_id"`
	Expiry          int64        `json:"expiry"`
	Amount          uint64       `json:"amount"`
	FeeReserve      uint64       `json:"fee_reserve" db:"fee_reserve"`
	FeePaid         uint64       `json:"paid_fee" db:"fee_paid"`
	SeenAt          int64        `json:"seen_at"`
	Melted          bool         `json:"melted"`
	Mpp             bool         `json:"mpp"`
}

func (meltRequest *MeltRequestDB) GetPostMeltQuoteResponse() PostMeltQuoteBolt11Response {
	return PostMeltQuoteBolt11Response{
		Quote:           meltRequest.Quote,
		Amount:          meltRequest.Amount,
		FeeReserve:      meltRequest.FeeReserve,
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
	Options PostMeltQuoteBolt11Options `json:"options"`
	Request string                     `json:"request"`
	Unit    string                     `json:"unit"`
}

func (p PostMeltQuoteBolt11Request) IsMpp() uint64 {
	if p.Options.Mpp["amount"] != 0 {
		return p.Options.Mpp["amount"]
	}
	return 0
}

type PostMeltQuoteBolt11Response struct {
	Quote           string           `json:"quote"`
	State           ACTION_STATE     `json:"state"`
	Unit            string           `json:"unit"`
	Request         string           `json:"request"`
	PaymentPreimage string           `json:"payment_preimage"`
	Change          []BlindSignature `json:"change"`
	Amount          uint64           `json:"amount"`
	FeeReserve      uint64           `json:"fee_reserve"`
	Expiry          int64            `json:"expiry"`
}

type PostMeltBolt11Request struct {
	Quote   string           `json:"quote"`
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

func (p *PostMeltBolt11Request) ValidateSigflag() error {
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

		if spendCondition.Data.Tags.originalTag != firstSpendCondition.Data.Tags.originalTag {
			return fmt.Errorf("not same tags %w", ErrInvalidSpendCondition)
		}

	}
	return nil
}

// makeSigAllMsg creates the message for SIG_ALL signature verification
// Format: secret_0 || C_0 || ... || secret_n || C_n || amount_0 || B_0 || ... || amount_m || B_m || quote_id
func (p *PostMeltBolt11Request) makeSigAllMsg() string {
	message := ""
	for _, proof := range p.Inputs {
		message = message + proof.Secret + proof.C.String()
	}
	for _, blindMessage := range p.Outputs {
		message = message + strconv.FormatUint(blindMessage.Amount, 10) + blindMessage.B_.String()
	}
	message = message + p.Quote
	return message
}
