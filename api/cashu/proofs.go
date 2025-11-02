package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
)

type ProofState string

const PROOF_UNSPENT ProofState = "UNSPENT"
const PROOF_SPENT ProofState = "SPENT"
const PROOF_PENDING ProofState = "PENDING"

type Proofs []Proof

func (p *Proofs) SetPendingAndQuoteRef(quote string) {
	for i := 0; i < len(*p); i++ {
		(*p)[i].State = PROOF_PENDING
		(*p)[i].Quote = &quote
	}
}
func (p *Proofs) Amount() uint64 {
	amount := uint64(0)
	for i := 0; i < len(*p); i++ {
		amount += (*p)[i].Amount
	}
	return amount
}

func (p *Proofs) SetProofsState(state ProofState) {
	for i := 0; i < len(*p); i++ {
		(*p)[i].State = state
	}
}

func (p *Proofs) SetQuoteReference(quote string) {
	for i := 0; i < len(*p); i++ {
		(*p)[i].Quote = &quote
	}
}

type Proof struct {
	Amount  uint64     `json:"amount"`
	Id      string     `json:"id"`
	Secret  string     `json:"secret"`
	C       string     `json:"C" db:"c"`
	Y       string     `json:"Y" db:"y"`
	Witness string     `json:"witness" db:"witness"`
	SeenAt  int64      `json:"seen_at"`
	State   ProofState `json:"state"`
	Quote   *string    `json:"quote" db:"quote"`
}

func (p Proof) VerifyP2PK(spendCondition *SpendCondition) (bool, error) {
	currentTime := time.Now().Unix()
	hashMessage := sha256.Sum256([]byte(p.Secret))
	witness, err := p.parseWitness()
	if err != nil {
		return false, fmt.Errorf("p.parseWitness(). %+v", err)
	}
	pubkeys, err := p.Pubkeys()
	if err != nil {
		return false, fmt.Errorf("p.Pubkeys(). %+v", err)

	}
	// check if locktime has passed and if there are refund keys
	if spendCondition.Data.Tags.Locktime != 0 && currentTime > int64(spendCondition.Data.Tags.Locktime) && len(spendCondition.Data.Tags.Refund) > 0 {
		refundPubkeys := make(map[*btcec.PublicKey]bool)
		for i := range spendCondition.Data.Tags.Refund {
			if spendCondition.Data.Tags.Refund[i] != nil {
				refundPubkeys[spendCondition.Data.Tags.Refund[i]] = true
			}
		}
		amountValidRefundSigs := 0
		for _, sig := range witness.Signatures {
			for pubkey, _ := range refundPubkeys {
				if sig.Verify(hashMessage[:], pubkey) {
					amountValidRefundSigs += 1
					delete(refundPubkeys, pubkey)
					continue
				}
			}
		}

		switch {
		case amountValidRefundSigs == 0:
			return false, ErrNoValidSignatures
		case spendCondition.Data.Tags.NSigRefund > 0 && amountValidRefundSigs < spendCondition.Data.Tags.NSigRefund:
			return false, ErrNotEnoughSignatures
		case spendCondition.Data.Tags.NSigRefund > 0 && amountValidRefundSigs >= spendCondition.Data.Tags.NSigRefund:
			return true, nil
		case amountValidRefundSigs >= 1:
			return true, nil
		default:
			return false, ErrLocktimePassed

		}
	}

	// append all posibles keys for signing
	amountValidSigs := 0
	for _, sig := range witness.Signatures {
		for pubkey, _ := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}

	// check if there is a multisig set up if not check if there is only one valid signature
	switch {
	case amountValidSigs == 0:
		return false, ErrNoValidSignatures
	case spendCondition.Data.Tags.NSigs > 0 && amountValidSigs < spendCondition.Data.Tags.NSigs:
		return false, ErrNotEnoughSignatures
	case spendCondition.Data.Tags.NSigs > 0 && amountValidSigs >= spendCondition.Data.Tags.NSigs:
		return true, nil
	case amountValidSigs >= 1:
		return true, nil
	default:
		return false, nil
	}
}

func (p Proof) VerifyHTLC(spendCondition *SpendCondition) (bool, error) {
	currentTime := time.Now().Unix()
	hashMessage := sha256.Sum256([]byte(p.Secret))
	witness, err := p.parseWitness()
	if err != nil {
		return false, fmt.Errorf("p.parseWitness(). %+v", err)
	}
	pubkeys, err := p.Pubkeys()
	if err != nil {
		return false, fmt.Errorf("p.Pubkeys(). %+v", err)
	}
	// check if locktime has passed and if there are refund keys
	if spendCondition.Data.Tags.Locktime != 0 && currentTime > int64(spendCondition.Data.Tags.Locktime) && len(spendCondition.Data.Tags.Refund) > 0 {
		refundPubkeys := make(map[*btcec.PublicKey]bool)
		for i := range spendCondition.Data.Tags.Refund {
			if spendCondition.Data.Tags.Refund[i] != nil {
				refundPubkeys[spendCondition.Data.Tags.Refund[i]] = true
			}
		}
		amountValidRefundSigs := 0
		for _, sig := range witness.Signatures {
			for pubkey, _ := range refundPubkeys {
				if sig.Verify(hashMessage[:], pubkey) {
					amountValidRefundSigs += 1
					delete(refundPubkeys, pubkey)
					continue
				}
			}
		}

		switch {
		case amountValidRefundSigs == 0:
			return false, ErrNoValidSignatures
		case spendCondition.Data.Tags.NSigRefund > 0 && amountValidRefundSigs < spendCondition.Data.Tags.NSigRefund:
			return false, ErrNotEnoughSignatures
		case spendCondition.Data.Tags.NSigRefund > 0 && amountValidRefundSigs >= spendCondition.Data.Tags.NSigRefund:
			return true, nil
		case amountValidRefundSigs >= 1:
			return true, nil
		default:
			return false, ErrLocktimePassed

		}
	}

	err = spendCondition.VerifyPreimage(witness)
	if err != nil {
		return false, fmt.Errorf("spendCondition.VerifyPreimage  %w ", err)
	}

	// append all posibles keys for signing
	amountValidSigs := 0
	for _, sig := range witness.Signatures {
		for pubkey, _ := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}

	// check if there is a multisig set up if not check if there is only one valid signature
	switch {
	case amountValidSigs == 0:
		return false, ErrNoValidSignatures
	case spendCondition.Data.Tags.NSigs > 0 && amountValidSigs < spendCondition.Data.Tags.NSigs:
		return false, ErrNotEnoughSignatures
	case spendCondition.Data.Tags.NSigs > 0 && amountValidSigs >= spendCondition.Data.Tags.NSigs:
		return true, nil
	case amountValidSigs >= 1:
		return true, nil
	default:
		return false, nil
	}
}

func (p Proof) Pubkeys() (map[*btcec.PublicKey]bool, error) {
	spendCondition, err := p.parseSpendCondition()
	if err != nil {
		return nil, err
	}

	pubkeysMap := make(map[*btcec.PublicKey]bool, 0)
	switch spendCondition.Type {
	case P2PK:
		spendConditionDataBytes, err := hex.DecodeString(spendCondition.Data.Data)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString(spendCondition.Data.Data). %w", err)
		}

		dataPubkey, err := btcec.ParsePubKey(spendConditionDataBytes)
		if err != nil {
			return nil, fmt.Errorf("btcec.ParsePubKey(spendConditionDataBytes). %w", err)
		}
		pubkeysMap[dataPubkey] = true
		if spendCondition.Data.Tags.Pubkeys != nil {
			for i := range spendCondition.Data.Tags.Pubkeys {
				if spendCondition.Data.Tags.Pubkeys[i] != nil {
					pubkeysMap[spendCondition.Data.Tags.Pubkeys[i]] = true
				}
			}
		}

	case HTLC:
		if spendCondition.Data.Tags.Pubkeys != nil {
			for i := range spendCondition.Data.Tags.Pubkeys {
				if spendCondition.Data.Tags.Pubkeys[i] != nil {
					pubkeysMap[spendCondition.Data.Tags.Pubkeys[i]] = true
				}
			}
		}
	}

	return pubkeysMap, nil
}

func (p Proof) parseSpendCondition() (*SpendCondition, error) {
	var spendCondition SpendCondition
	err := json.Unmarshal([]byte(p.Secret), &spendCondition)

	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal([]byte(p.Secret), &spendCondition)  %w, %w", ErrCouldNotParseSpendCondition, err)
	}
	return &spendCondition, nil
}
func (p Proof) parseWitness() (*Witness, error) {
	var witness Witness
	err := json.Unmarshal([]byte(p.Witness), &witness)
	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal([]byte(p.Witness), &witness)  %w, %w", ErrCouldNotParseWitness, err)
	}

	return &witness, nil
}

func (p Proof) IsProofSpendConditioned() (bool, *SpendCondition, error) {
	var rawJsonSecret []json.RawMessage
	if err := json.Unmarshal([]byte(p.Secret), &rawJsonSecret); err != nil {
		return false, nil, nil
	}

	// Well-known secret should have a length of at least 2
	if len(rawJsonSecret) < 2 {
		return false, nil, errors.New("invalid secret: length < 2")
	}

	var kind string
	if err := json.Unmarshal(rawJsonSecret[0], &kind); err != nil {
		return false, nil, fmt.Errorf("json.Unmarshal(rawJsonSecret[0], &kind);%w", err)
	}

	if kind != "P2PK" && kind != "HTLC" {
		return false, nil, nil
	}

	spendCondition, err := p.parseSpendCondition()
	if err != nil {
		return false, nil, fmt.Errorf("p.parseSpendCondition(). %w", err)
	}

	return true, spendCondition, nil
}

func (p Proof) HashSecretToCurve() (Proof, error) {

	// Get Hash to curve of secret
	parsedProof := []byte(p.Secret)

	y, err := crypto.HashToCurve(parsedProof)

	if err != nil {
		return p, fmt.Errorf("crypto.HashToCurve: %+v", err)
	}

	Y_hex := hex.EncodeToString(y.SerializeCompressed())
	p.Y = Y_hex
	return p, nil
}
func (p *Proof) Sign(privkey *secp256k1.PrivateKey) error {
	hash := sha256.Sum256([]byte(p.Secret))

	sig, err := schnorr.Sign(privkey, hash[:])
	if err != nil {
		return fmt.Errorf("schnorr.Sign: %w", err)
	}

	var witness Witness
	if p.Witness == "" {
		witness = Witness{}
	} else {
		err = json.Unmarshal([]byte(p.Witness), &witness)
		if err != nil {
			return fmt.Errorf("json.Unmarshal([]byte(p.Witness), &witness)  %w, %w", ErrCouldNotParseWitness, err)
		}
	}

	witness.Signatures = append(witness.Signatures, sig)

	witnessStr, err := witness.String()

	if err != nil {
		return fmt.Errorf("witness.String: %w", err)
	}

	p.Witness = witnessStr
	return nil
}
func (p *Proof) AddPreimage(preimage string) error {

	var witness Witness
	if p.Witness == "" {
		witness = Witness{}
	} else {
		err := json.Unmarshal([]byte(p.Witness), &witness)
		if err != nil {
			return fmt.Errorf("json.Unmarshal([]byte(p.Witness), &witness)  %w, %w", ErrCouldNotParseWitness, err)
		}
	}

	witness.Preimage = preimage

	witnessStr, err := witness.String()

	if err != nil {
		return fmt.Errorf("witness.String: %w", err)
	}

	p.Witness = witnessStr
	return nil
}
