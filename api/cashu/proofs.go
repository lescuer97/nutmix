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
	Amount  uint64           `json:"amount"`
	Id      string           `json:"id"`
	Secret  string           `json:"secret"`
	C       WrappedPublicKey `json:"C" db:"c"`
	Y       WrappedPublicKey `json:"Y" db:"y"`
	Witness string           `json:"witness" db:"witness"`
	SeenAt  int64            `json:"seen_at"`
	State   ProofState       `json:"state"`
	Quote   *string          `json:"quote" db:"quote"`
}

func (p Proof) verifyP2PKSpendCondition(spendCondition *SpendCondition, witness *Witness) (bool, error) {
	pubkeys, err := p.pubkeysForVerification(spendCondition)
	if err != nil {
		return false, fmt.Errorf("p.pubkeysForVerification(spendCondition). %w", err)
	}
	nsigToCheck := uint(1)
	if spendCondition.Data.Tags.NSigs > nsigToCheck {
		nsigToCheck = spendCondition.Data.Tags.NSigs
	}

	amountValidSigs := uint(0)
	hashMessage := sha256.Sum256([]byte(p.Secret))
	for _, sig := range witness.Signatures {
		for pubkey := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}
	switch {
	case amountValidSigs == 0:
		return false, ErrNoValidSignatures
	case nsigToCheck > 0 && amountValidSigs < nsigToCheck:
		return false, ErrNotEnoughSignatures
	case nsigToCheck > 0 && amountValidSigs >= nsigToCheck:
		return true, nil
	case amountValidSigs >= 1:
		return true, nil
	default:
		return false, nil
	}
}

func (p Proof) VerifyP2PK(spendCondition *SpendCondition) (bool, error) {
	witness, err := p.parseWitness()
	if err != nil {
		return false, fmt.Errorf("p.parseWitness(). %+v", err)
	}
	valid, err := p.verifyP2PKSpendCondition(spendCondition, witness)
	if err != nil {
		if errors.Is(err, ErrNoValidSignatures) || errors.Is(err, ErrNotEnoughSignatures) {
		} else {
			return false, fmt.Errorf("p.verifyP2PKSpendCondition(spendCondition, witness). %w", err)
		}
	}
	if valid {
		return true, nil
	}
	if p.timelockPassed(spendCondition) {
		valid, err = p.verifyTimelockPassedSpendCondition(spendCondition, witness)
	}
	return valid, err
}

func (p Proof) verifyHtlcSpendCondition(spendCondition *SpendCondition, witness *Witness) (bool, error) {
	err := spendCondition.VerifyPreimage(witness)
	if err != nil {
		return false, fmt.Errorf("spendCondition.VerifyPreimage(witness). %w", err)
	}

	if len(spendCondition.Data.Tags.Pubkeys) == 0 {
		return false, ErrInvalidPreimage
	}

	pubkeys, err := p.pubkeysForVerification(spendCondition)
	if err != nil {
		return false, fmt.Errorf("p.pubkeysForVerification(spendCondition). %w", err)
	}
	nsigToCheck := uint(1)
	if spendCondition.Data.Tags.NSigs > nsigToCheck {
		nsigToCheck = spendCondition.Data.Tags.NSigs
	}

	hashMessage := sha256.Sum256([]byte(p.Secret))
	amountValidSigs := uint(0)
	for _, sig := range witness.Signatures {
		for pubkey := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}

	switch {
	case amountValidSigs == 0:
		return false, ErrNoValidSignatures
	case nsigToCheck > 0 && amountValidSigs < nsigToCheck:
		return false, ErrNotEnoughSignatures
	case nsigToCheck > 0 && amountValidSigs >= nsigToCheck:
		return true, nil
	case amountValidSigs >= 1:
		return true, nil
	default:
		return false, nil
	}
}

func (p Proof) VerifyHTLC(spendCondition *SpendCondition) (bool, error) {
	witness, err := p.parseWitness()
	if err != nil {
		return false, fmt.Errorf("p.parseWitness(). %+v", err)
	}

	valid, err := p.verifyHtlcSpendCondition(spendCondition, witness)
	if err != nil {
		if errors.Is(err, ErrNoValidSignatures) || errors.Is(err, ErrNotEnoughSignatures) {
		} else {
			return false, fmt.Errorf("p.verifyP2PKSpendCondition(spendCondition, witness). %w", err)
		}
	}
	if valid {
		return true, nil
	}
	if p.timelockPassed(spendCondition) {
		valid, err = p.verifyTimelockPassedSpendCondition(spendCondition, witness)
	}
	return valid, err
}

func (p Proof) timelockPassed(spendCondition *SpendCondition) bool {
	currentTime := time.Now().Unix()
	return spendCondition.Data.Tags.Locktime != 0 && currentTime > int64(spendCondition.Data.Tags.Locktime)
}

func (p Proof) verifyTimelockPassedSpendCondition(spendCondition *SpendCondition, witness *Witness) (bool, error) {
	pubkeys, err := p.pubkeysForRefund(spendCondition)
	if err != nil {
		return false, fmt.Errorf("p.pubkeysForRefund(spendCondition). %w", err)
	}

	nsigToCheck := uint(0)
	if len(spendCondition.Data.Tags.Refund) > 0 {
		nsigToCheck = 1
	}

	if spendCondition.Data.Tags.NSigRefund > nsigToCheck {
		nsigToCheck = spendCondition.Data.Tags.NSigRefund
	}

	amountValidSigs := uint(0)
	hashMessage := sha256.Sum256([]byte(p.Secret))
	for _, sig := range witness.Signatures {
		for pubkey := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}
	switch {
	case amountValidSigs == 0:
		return false, ErrNoValidSignatures
	case nsigToCheck > 0 && amountValidSigs < nsigToCheck:
		return false, ErrNotEnoughSignatures
	case nsigToCheck > 0 && amountValidSigs >= nsigToCheck:
		return true, nil
	case amountValidSigs >= 1:
		return true, nil
	default:
		return false, nil
	}
}

func (p Proof) pubkeysForVerification(spendCondition *SpendCondition) (map[*btcec.PublicKey]struct{}, error) {
	pubkeysMap := make(map[*btcec.PublicKey]struct{}, 0)
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
		pubkeysMap[dataPubkey] = struct{}{}
		if spendCondition.Data.Tags.Pubkeys != nil {
			for i := range spendCondition.Data.Tags.Pubkeys {
				if spendCondition.Data.Tags.Pubkeys[i] != nil {
					pubkeysMap[spendCondition.Data.Tags.Pubkeys[i]] = struct{}{}
				}
			}
		}
	case HTLC:
		if spendCondition.Data.Tags.Pubkeys != nil {
			for i := range spendCondition.Data.Tags.Pubkeys {
				if spendCondition.Data.Tags.Pubkeys[i] != nil {
					pubkeysMap[spendCondition.Data.Tags.Pubkeys[i]] = struct{}{}
				}
			}
		}
	}
	return pubkeysMap, nil
}

func (p Proof) pubkeysForRefund(spendCondition *SpendCondition) (map[*btcec.PublicKey]struct{}, error) {
	pubkeysMap := make(map[*btcec.PublicKey]struct{}, 0)
	for i := range spendCondition.Data.Tags.Refund {
		if spendCondition.Data.Tags.Refund[i] != nil {
			pubkeysMap[spendCondition.Data.Tags.Refund[i]] = struct{}{}
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

	p.Y = WrappedPublicKey{y}
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

func (p *Proof) UnmarshalJSON(data []byte) error {
	// Define an alias to avoid infinite recursion
	type Alias Proof
	aux := &struct {
		C string `json:"C"` // Temporarily hold C as a string
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	// Unmarshal into the auxiliary struct
	if err := json.Unmarshal(data, aux); err != nil {
		return errors.Join(ErrInvalidProof, err)
	}

	// Decode the hex string into bytes
	decoded, err := hex.DecodeString(aux.C)
	if err != nil {
		return errors.Join(ErrInvalidProof, err)
	}

	// Parse the bytes into a secp256k1.PublicKey
	pubKey, err := secp256k1.ParsePubKey(decoded)
	if err != nil {
		return errors.Join(ErrInvalidProof, err)
	}

	// Assign the parsed public key to the struct
	p.C = WrappedPublicKey{PublicKey: pubKey}
	return nil
}

// VerifyProofsSpendConditions verifies P2PK and HTLC conditions for each proof individually.
func VerifyProofsSpendConditions(proofs Proofs) error {
	for _, proof := range proofs {
		isLocked, spendCondition, err := proof.IsProofSpendConditioned()
		if err != nil {
			return fmt.Errorf("proof.IsProofSpendConditioned(). %+v", err)
		}
		if isLocked {

			err = spendCondition.CheckValid()
			if err != nil {
				return fmt.Errorf("spendCondition.CheckValid(). %w", err)
			}
			switch spendCondition.Type {
			case P2PK:
				valid, err := proof.VerifyP2PK(spendCondition)
				if err != nil {
					return fmt.Errorf("proof.VerifyP2PK(spendCondition). %w", err)
				}
				if !valid {
					return ErrInvalidSpendCondition
				}
			case HTLC:
				valid, err := proof.VerifyHTLC(spendCondition)
				if err != nil {
					return fmt.Errorf("proof.VerifyHTLC(spendCondition). %w", err)
				}
				if !valid {
					return ErrInvalidSpendCondition
				}
			}

		}
		if !isLocked {
			if len(proof.Secret) != 64 {
				return ErrCommonSecretNotCorrectSize
			}
		}
	}
	return nil
}
