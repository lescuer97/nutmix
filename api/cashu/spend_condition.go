package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

var (
	ErrInvalidSpendCondition         = errors.New("invalid spend condition")
	ErrConvertSpendConditionToString = errors.New("failed to convert spend condition to string")
	ErrInvalidTagName                = errors.New("invalid tag name")
	ErrConvertTagToString            = errors.New("failed to convert tag to string")
	ErrInvalidTagValue               = errors.New("invalid tag value")
	ErrInvalidSigFlag                = errors.New("invalid sig flag")
	ErrConvertSigFlagToString        = errors.New("failed to convert sigu flag to string")
	ErrMalformedTag                  = errors.New("malformed tag")
	ErrCouldNotParseSpendCondition   = errors.New("could not parse spend condition")
	ErrCouldNotParseWitness          = errors.New("could not parse witness")
	ErrEmptyWitness                  = errors.New("witness is empty")
	ErrNoValidSignatures             = errors.New("no valid signatures found")
	ErrNotEnoughSignatures           = errors.New("not enough signatures")
	ErrLocktimePassed                = errors.New("locktime has passed and no refund key was found")
	ErrInvalidHexPreimage            = errors.New("preimage is not a valid hex string")
	ErrInvalidPreimage               = errors.New("invalid preimage")
)

type SpendCondition struct {
	Data SpendConditionData
	Type SpendConditionType
}

func (s *SpendCondition) UnmarshalJSON(b []byte) error {
	a := []interface{}{&s.Type, &s.Data}
	return json.Unmarshal(b, &a)
}

// MarshalJSON serializes SpendCondition to cashu protocol format: ["P2PK",{"nonce":"...","data":"...","tags":[...]}]
func (sc *SpendCondition) MarshalJSON() ([]byte, error) {
	// AnyOneCanSpend cannot be marshalled to JSON (it's a plain 64-byte secret)
	if sc.Type == AnyOneCanSpend {
		return nil, fmt.Errorf("cannot marshal AnyOneCanSpend to JSON: %w", ErrInvalidSpendCondition)
	}

	typeStr, err := sc.Type.String()
	if err != nil {
		return nil, fmt.Errorf("sc.Type.String(): %w", err)
	}

	dataJSON, err := sc.Data.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("sc.Data.MarshalJSON(): %w", err)
	}

	// Format: ["TYPE", {...data...}]
	result := fmt.Sprintf(`["%s",%s]`, typeStr, string(dataJSON))
	return []byte(result), nil
}

// String returns the JSON string representation of the SpendCondition
func (sc *SpendCondition) String() (string, error) {
	b, err := sc.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (sc *SpendCondition) CheckValid() error {
	if len(sc.Data.Tags.Pubkeys)+len(sc.Data.Tags.Refund) > 10 {
		return ErrInvalidSpendCondition
	}

	return nil
}

// HasSigAll returns true if this spend condition requires SIG_ALL flag
func (sc *SpendCondition) HasSigAll() bool {
	return sc.Data.Tags.Sigflag == SigAll
}

type SpendConditionType int

const (
	AnyOneCanSpend SpendConditionType = iota // 0 - non-JSON 64-byte hex secret
	P2PK                                     // 1
	HTLC                                     // 2
)

func (sc *SpendConditionType) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"P2PK"`, "P2PK":
		*sc = P2PK
	case `"HTLC"`, "HTLC":
		*sc = HTLC
	default:
		// For non-JSON secrets or unknown types, treat as AnyOneCanSpend
		*sc = AnyOneCanSpend
	}
	return nil
}

func (sc SpendConditionType) String() (string, error) {
	switch sc {
	case AnyOneCanSpend:
		return "", nil // AnyOneCanSpend has no string representation (plain 64-byte secret)
	case P2PK:
		return "P2PK", nil
	case HTLC:
		return "HTLC", nil
	default:
		return "", ErrConvertSpendConditionToString
	}
}

// IsSpendConditioned returns true if this type requires spend condition verification
func (sc SpendConditionType) IsSpendConditioned() bool {
	return sc == P2PK || sc == HTLC
}

type TagsInfo struct {
	originalTag string
	Pubkeys     []*btcec.PublicKey
	Refund      []*btcec.PublicKey
	Sigflag     SigFlag
	NSigs       uint
	Locktime    uint
	NSigRefund  uint
}

// MarshalJSON serializes TagsInfo to the cashu protocol format: [["sigflag","SIG_ALL"],["pubkeys","..."]]
// Per NUT-11 spec: Tags are arrays with format ["key", "value1", "value2", ...]
// Note: Integer tag values (n_sigs, locktime, n_sigs_refund) are serialized as strings per spec
func (tags *TagsInfo) MarshalJSON() ([]byte, error) {
	var result [][]string

	// Add sigflag if set (if unset/0, defaults to SIG_INPUTS per NUT-11 spec and is omitted)
	if tags.Sigflag != 0 {
		result = append(result, []string{"sigflag", tags.Sigflag.String()})
	}

	// Add pubkeys if present
	if len(tags.Pubkeys) > 0 {
		pubkeysTag := []string{"pubkeys"}
		for _, pubkey := range tags.Pubkeys {
			pubkeysTag = append(pubkeysTag, hex.EncodeToString(pubkey.SerializeCompressed()))
		}
		result = append(result, pubkeysTag)
	}

	// Add n_sigs if set (greater than 0)
	if tags.NSigs > 0 {
		result = append(result, []string{"n_sigs", strconv.FormatUint(uint64(tags.NSigs), 10)})
	}

	// Add locktime if set
	if tags.Locktime > 0 {
		result = append(result, []string{"locktime", strconv.FormatUint(uint64(tags.Locktime), 10)})
	}

	// Add refund pubkeys if present
	if len(tags.Refund) > 0 {
		refundTag := []string{"refund"}
		for _, pubkey := range tags.Refund {
			refundTag = append(refundTag, hex.EncodeToString(pubkey.SerializeCompressed()))
		}
		result = append(result, refundTag)
	}

	// Add n_sigs_refund if set
	if tags.NSigRefund > 0 {
		result = append(result, []string{"n_sigs_refund", strconv.FormatUint(uint64(tags.NSigRefund), 10)})
	}

	return json.Marshal(result)
}

func (tags *TagsInfo) UnmarshalJSON(b []byte) error {

	var arrayToCheck [][]string

	err := json.Unmarshal(b, &arrayToCheck)

	if err != nil {
		return fmt.Errorf("json.Unmarshal(b, &arrayToCheck): %w", err)
	}

	for _, tag := range arrayToCheck {

		if len(tag) < 2 {
			return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
		}

		tagName, err := TagFromString(tag[0])

		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidTagName, tag[0])
		}

		tagInfo := tag[1:]
		switch tagName {

		case Sigflag:
			if len(tagInfo) != 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			sigFlag, err := SigFlagFromString(tagInfo[0])
			if err != nil {
				return errors.Join(ErrInvalidSigFlag, err)
			}

			tags.Sigflag = sigFlag

		case Pubkeys, Refund:
			if len(tagInfo) < 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			for _, pubkey := range tagInfo {
				bytesPubkey, err := hex.DecodeString(pubkey)
				if err != nil {
					return fmt.Errorf("hex.DecodeString: %w", err)
				}

				parsedPubkey, err := btcec.ParsePubKey(bytesPubkey)
				if err != nil {
					return fmt.Errorf("secp256k1.ParsePubKey: %w", err)
				}

				switch tagName {
				case Pubkeys:
					tags.Pubkeys = append(tags.Pubkeys, parsedPubkey)

				case Refund:
					tags.Refund = append(tags.Refund, parsedPubkey)
				}

			}

		case NSigs:
			if len(tagInfo) != 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			nSigs, err := strconv.ParseUint(tagInfo[0], 10, 64)
			if err != nil {
				return fmt.Errorf("strconv.ParseUint: %s: %w", tagInfo[0], err)
			}

			tags.NSigs = uint(nSigs)

		case NSigRefund:
			if len(tagInfo) != 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			nSigsRefund, err := strconv.ParseUint(tagInfo[0], 10, 64)
			if err != nil {
				return fmt.Errorf("strconv.ParseUint: %s: %w", tagInfo[0], err)
			}

			tags.NSigRefund = uint(nSigsRefund)

		case Locktime:
			if len(tagInfo) != 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			locktime, err := strconv.ParseUint(tagInfo[0], 10, 64)
			if err != nil {
				return fmt.Errorf("strconv.ParseUint: %s: %w", tagInfo[0], err)
			}

			tags.Locktime = uint(locktime)
		}

	}
	tags.originalTag = string(b)
	return nil
}

type SpendConditionData struct {
	Nonce string
	Data  string
	Tags  TagsInfo
}

// MarshalJSON serializes SpendConditionData to: {"nonce":"...","data":"...","tags":[...]}
func (scd *SpendConditionData) MarshalJSON() ([]byte, error) {
	// Create a map for controlled serialization order
	tagsJSON, err := scd.Tags.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("scd.Tags.MarshalJSON(): %w", err)
	}

	// Build the JSON manually to ensure correct key order and format
	result := fmt.Sprintf(`{"nonce":"%s","data":"%s","tags":%s}`, scd.Nonce, scd.Data, string(tagsJSON))
	return []byte(result), nil
}

func (sc *SpendCondition) VerifyPreimage(witness *Witness) error {
	preImageBytes, err := hex.DecodeString(witness.Preimage)

	if err != nil {
		return errors.Join(ErrInvalidHexPreimage, err)
	}

	parsedPreimage := sha256.Sum256(preImageBytes)

	if hex.EncodeToString(parsedPreimage[:]) != sc.Data.Data {
		return ErrInvalidPreimage
	}

	return nil

}

type Tags int

const (
	Sigflag    Tags = iota + 1 // 1
	Pubkeys                    // 2
	NSigs                      // 3
	Locktime                   // 4
	Refund                     // 5
	NSigRefund                 // 6
)

func (t Tags) String() string {
	switch t {
	case Sigflag:
		return "sigflag"
	case Pubkeys:
		return "pubkeys"
	case NSigs:
		return "n_sigs"
	case Locktime:
		return "locktime"
	case Refund:
		return "refund"
	case NSigRefund:
		return "n_sigs_refund"
	}
	return ""
}

func TagFromString(s string) (Tags, error) {
	switch s {
	case "sigflag":
		return Sigflag, nil
	case "pubkeys":
		return Pubkeys, nil
	case "n_sigs":
		return NSigs, nil
	case "locktime":
		return Locktime, nil
	case "refund":
		return Refund, nil
	case "n_sigs_refund":
		return NSigRefund, nil
	default:
		return 0, ErrInvalidTagName
	}
}

type SigFlag int

const (
	SigAll    SigFlag = iota + 1 // 1
	SigInputs                    // 2
)

func (sf SigFlag) String() string {
	switch sf {
	case SigAll:
		return "SIG_ALL"
	case SigInputs:
		return "SIG_INPUTS"
	}
	return ""
}

func SigFlagFromString(s string) (SigFlag, error) {
	switch s {
	case "SIG_ALL":
		return SigAll, nil
	case "SIG_INPUTS":
		return SigInputs, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrInvalidTagValue, s)
	}
}

type Witness struct {
	Preimage   string `json:"preimage,omitempty"`
	Signatures []*schnorr.Signature
}

func (wit *Witness) String() (string, error) {
	var witness = struct {
		Preimage   string
		Signatures []string
	}{}

	for _, sig := range wit.Signatures {
		witness.Signatures = append(witness.Signatures, hex.EncodeToString(sig.Serialize()))
	}

	if wit.Preimage != "" {
		witness.Preimage = wit.Preimage
	}

	b, err := json.Marshal(witness)
	if err != nil {
		return "", fmt.Errorf("json.Marshal(signatures): %w", err)
	}
	return string(b), nil
}

func (wit *Witness) UnmarshalJSON(b []byte) error {
	var sigs = struct {
		Preimage   string
		Signatures []string
	}{}

	err := json.Unmarshal(b, &sigs)

	if err != nil {
		return fmt.Errorf("json.Unmarshal(b, &info): %w", err)
	}

	witness := Witness{
		Signatures: make([]*schnorr.Signature, 0),
	}
	if sigs.Preimage != "" {
		witness.Preimage = sigs.Preimage
	}

	for _, sig := range sigs.Signatures {
		sigBytes, err := hex.DecodeString(sig)
		if err != nil {
			return fmt.Errorf("hex.DecodeString: %w", err)
		}
		signature, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			return fmt.Errorf("schnorr.ParseSignature(sigBytes): %w", err)
		}

		witness.Signatures = append(witness.Signatures, signature)

	}

	*wit = witness

	return nil

}

type SigflagValidation struct {
	pubkeys                  map[string]struct{}
	refundPubkeys            map[string]struct{}
	sigFlag                  SigFlag
	signaturesRequired       uint
	signaturesRequiredRefund uint
}

func checkForSigAll(proofs Proofs) (SigflagValidation, error) {
	sigflagValidation := SigflagValidation{
		sigFlag:                  SigInputs,
		signaturesRequired:       1,
		signaturesRequiredRefund: 0,
		pubkeys:                  make(map[string]struct{}),
		refundPubkeys:            make(map[string]struct{}),
	}
	for _, proof := range proofs {
		isLocked, spendCondition, err := proof.IsProofSpendConditioned()
		if err != nil {
			return SigflagValidation{}, fmt.Errorf("proof.parseSpendCondition(). %w", err)
		}
		if isLocked && spendCondition != nil {
			if spendCondition.Data.Tags.Sigflag == SigAll {
				sigflagValidation.sigFlag = SigAll
				if spendCondition.Data.Tags.NSigs > 1 {
					sigflagValidation.signaturesRequired = spendCondition.Data.Tags.NSigs
				}
				pubkeys, err := proof.pubkeysForVerification(spendCondition)
				if err != nil {
					return SigflagValidation{}, fmt.Errorf("proof.pubkeysForVerification(spendCondition). %w", err)
				}

				sigflagValidation.pubkeys = pubkeys
				if len(spendCondition.Data.Tags.Refund) > 0 {
					sigflagValidation.signaturesRequiredRefund = 1
					if spendCondition.Data.Tags.NSigRefund > 1 {
						sigflagValidation.signaturesRequiredRefund = spendCondition.Data.Tags.NSigRefund
					}
					sigflagValidation.refundPubkeys = proof.pubkeysForRefund(spendCondition)
				}
				return sigflagValidation, nil
			}
		}
	}
	return sigflagValidation, nil
}

// ProofsHaveSigAll returns true if any proof in the slice has SIG_ALL flag set.
// This is useful for determining if outputs need to be included in signature verification.
func ProofsHaveSigAll(proofs Proofs) (bool, error) {
	for _, proof := range proofs {
		isLocked, spendCondition, err := proof.IsProofSpendConditioned()
		if err != nil {
			return false, fmt.Errorf("proof.IsProofSpendConditioned(): %w", err)
		}
		if isLocked && spendCondition != nil && spendCondition.HasSigAll() {
			return true, nil
		}
	}
	return false, nil
}

func checkValidSignature(msg string, pubkeys map[string]struct{}, signatures []*schnorr.Signature) (uint, error) {
	hashMessage := sha256.Sum256([]byte(msg))
	amountValidSigs := uint(0)

	for _, sig := range signatures {
		for pubkey := range pubkeys {
			parsedPubkey, err := btcec.ParsePubKey([]byte(pubkey))
			if err != nil {
				return 0, fmt.Errorf("btcec.ParsePubKey([]byte(pubkey)). %w", err)
			}
			if sig.Verify(hashMessage[:], parsedPubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}
	return amountValidSigs, nil
}
