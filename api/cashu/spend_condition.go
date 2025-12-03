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
	ErrInvalidSpendCondition         = errors.New("Invalid spend condition")
	ErrConvertSpendConditionToString = errors.New("Failed to convert spend condition to string")
	ErrInvalidTagName                = errors.New("Invalid tag name")
	ErrConvertTagToString            = errors.New("Failed to convert tag to string")
	ErrInvalidTagValue               = errors.New("Invalid tag value")
	ErrInvalidSigFlag                = errors.New("Invalid sig flag")
	ErrConvertSigFlagToString        = errors.New("Failed to convert sig flag to string")
	ErrMalformedTag                  = errors.New("Malformed tag")
	ErrCouldNotParseSpendCondition   = errors.New("Could not parse spend condition")
	ErrCouldNotParseWitness          = errors.New("Could not parse witness")
	ErrEmptyWitness                  = errors.New("Witness is empty")
	ErrNoValidSignatures             = errors.New("No valid signatures found")
	ErrNotEnoughSignatures           = errors.New("Not enough signatures")
	ErrLocktimePassed                = errors.New("Locktime has passed and no refund key was found")
	ErrInvalidHexPreimage            = errors.New("Preimage is not a valid hex string")
	ErrInvalidPreimage               = errors.New("Invalid preimage")
)

type SpendCondition struct {
	Type SpendConditionType
	Data SpendConditionData
}

func (s *SpendCondition) UnmarshalJSON(b []byte) error {
	a := []interface{}{&s.Type, &s.Data}
	return json.Unmarshal(b, &a)
}

// ["P2PK",{"nonce":"3229136a6627050449e85dcdf90315f87519f172b2af80b2e1d460695db511ab","data":"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae"}]
func (sc *SpendCondition) MarshalJSON() ([]byte, error) {
	str := "["

	typestr, err := sc.Type.String()

	if err != nil {
		return nil, err
	}

	str += fmt.Sprintf("\"%s\",", typestr)

	str += "{"
	str += fmt.Sprintf("\"%s\",", sc.Data.Nonce)

	return []byte(str), nil
}

func (sc *SpendCondition) String() (string, error) {
	str := "["

	typestr, err := sc.Type.String()

	if err != nil {
		return "", err
	}

	str += fmt.Sprintf("\"%s\",", typestr)
	str += fmt.Sprintf(`{"nonce":"%s",`, sc.Data.Nonce)
	str += fmt.Sprintf(`"data":"%s",`, sc.Data.Data)
	str += fmt.Sprintf(`"tags":[`)
	str += fmt.Sprintf(`["sigflag","%s"],`, sc.Data.Tags.Sigflag.String())
	str += fmt.Sprintf(`["n_sigs","%s"],`, strconv.Itoa(sc.Data.Tags.NSigs))
	str += fmt.Sprintf(`["locktime","%s"],`, strconv.Itoa(sc.Data.Tags.Locktime))
	if len(sc.Data.Tags.Refund) > 0 {

		str += fmt.Sprintf(`["refund"`)

		for _, pubkey := range sc.Data.Tags.Refund {

			str += fmt.Sprintf(`,"%s"`, hex.EncodeToString(pubkey.SerializeCompressed()))
		}

		str += fmt.Sprintf(`],`)

	}
	if len(sc.Data.Tags.Pubkeys) > 0 {

		str += fmt.Sprintf(`["pubkeys"`)

		for _, pubkey := range sc.Data.Tags.Pubkeys {

			str += fmt.Sprintf(`,"%s"`, hex.EncodeToString(pubkey.SerializeCompressed()))
		}

		str += fmt.Sprintf(`]`)

	}

	str += fmt.Sprintf(`]}]`)

	return str, nil
}

func (sc *SpendCondition) CheckValid() error {
	if len(sc.Data.Tags.Pubkeys)+len(sc.Data.Tags.Pubkeys) > 10 {
		return ErrInvalidSpendCondition
	}

	return nil
}

type SpendConditionType int

const (
	P2PK SpendConditionType = iota + 1
	HTLC SpendConditionType = iota + 2
)

func (sc *SpendConditionType) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"P2PK"`, "P2PK":
		*sc = P2PK
		break
	case `"HTLC"`, "HTLC":
		*sc = HTLC
		break

	default:
		return ErrInvalidSpendCondition
	}
	return nil

}
func (sc SpendConditionType) String() (string, error) {
	switch sc {
	case P2PK:
		return "P2PK", nil
	case HTLC:
		return "HTLC", nil
	default:
		return "", ErrConvertSpendConditionToString
	}
}

type TagsInfo struct {
	originalTag string
	Sigflag     SigFlag
	Pubkeys     []*btcec.PublicKey
	NSigs       int
	Locktime    int
	Refund      []*btcec.PublicKey
	NSigRefund  int
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
				return  errors.Join(ErrInvalidSigFlag, err)
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

			nSigs, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return fmt.Errorf("strconv.Atoi: %w", err)
			}

			tags.NSigs = nSigs

		case Locktime:
			if len(tagInfo) != 1 {
				return fmt.Errorf("%w: %s", ErrMalformedTag, tag)
			}

			locktime, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return fmt.Errorf("strconv.Atoi: %w", err)
			}

			tags.Locktime = locktime
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

func (sc *SpendCondition) VerifyPreimage(witness *Witness) error {
	preImageBytes, err := hex.DecodeString(witness.Preimage)

	if err != nil {
		return errors.Join(ErrInvalidHexPreimage, err)
	}

	if len(preImageBytes) != 32 {
		return ErrInvalidPreimage
	}

	parsedPreimage := sha256.Sum256(preImageBytes)

	if hex.EncodeToString(parsedPreimage[:]) != sc.Data.Data {
		return ErrInvalidPreimage
	}

	return nil

}

type Tags int

const (
	Sigflag    Tags = iota + 1
	Pubkeys    Tags = iota + 2
	NSigs      Tags = iota + 3
	Locktime   Tags = iota + 4
	Refund     Tags = iota + 5
	NSigRefund Tags = iota + 6
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
		return "refund"
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
	SigAll    SigFlag = iota + 1
	SigInputs SigFlag = iota + 2
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
		return "", fmt.Errorf("json.Marshal(singatures): %w", err)
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
	sigFlag            SigFlag
	signaturesRequired uint
	pubkeys            map[*btcec.PublicKey]bool
}

func checkForSigAll(proofs Proofs) (SigflagValidation, error) {
	sigFlagValidation := SigflagValidation{
		sigFlag:            SigInputs,
		signaturesRequired: 0,
		pubkeys:            make(map[*btcec.PublicKey]bool),
	}
	for _, proof := range proofs {
		isLocked, spendCondition, err := proof.IsProofSpendConditioned()
		if err != nil {
			return sigFlagValidation, fmt.Errorf("proof.parseSpendCondition(). %w", err)
		}
		if isLocked && spendCondition != nil {
			if spendCondition.Data.Tags.Sigflag == SigAll {
				sigFlagValidation.sigFlag = SigAll
			}
			if sigFlagValidation.signaturesRequired < uint(spendCondition.Data.Tags.NSigs) {
				sigFlagValidation.signaturesRequired = uint(spendCondition.Data.Tags.NSigs)
			}

			for _, pubkey := range spendCondition.Data.Tags.Pubkeys {
				sigFlagValidation.pubkeys[pubkey] = true
			}
		}
	}
	return sigFlagValidation, nil
}

func checkValidSignature(msg string, pubkeys map[*btcec.PublicKey]bool, signatures []*schnorr.Signature) (uint, error) {
	hashMessage := sha256.Sum256([]byte(msg))
	amountValidSigs := uint(0)

	for _, sig := range signatures {
		for pubkey, _ := range pubkeys {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
				delete(pubkeys, pubkey)
				continue
			}
		}
	}
	return amountValidSigs, nil
}

// func GetSignaturesFromWitness(proof Proof) ([]*schnorr.Signature, error){
//
// }
// func MakeSigAllMessage(proofs Proofs) (string, error){
// 	for _, proof := range proofs{
// 	}
//
// }
