package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

const (
	ErrInvalidSpendCondition         = "Invalid spend condition"
	ErrConvertSpendConditionToString = "Failed to convert spend condition to string"
	ErrInvalidTagName                = "Invalid tag name"
	ErrConvertTagToString            = "Failed to convert tag to string"
	ErrInvalidTagValue               = "Invalid tag value"
	ErrInvalidSigFlag                = "Invalid sig flag"
	ErrConvertSigFlagToString        = "Failed to convert sig flag to string"
	ErrMalformedTag                  = "Malformed tag"
	ErrCouldNotParseSpendCondition   = "Could not parse spend condition"
	ErrCouldNotParseWitness          = "Could not parse witness"
)

type SpendCondition struct {
	Type SpendConditionType
	Data SpendConditionData
}

func (s *SpendCondition) UnmarshalJSON(b []byte) error {
	a := []interface{}{&s.Type, &s.Data}
	return json.Unmarshal(b, &a)
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
		return errors.New(ErrInvalidSpendCondition)
	}
	return nil

}

type TagsInfo struct {
	Sigflag  SigFlag
	Pubkeys  []*btcec.PublicKey
	NSigs    int
	Locktime int
	Refund   []*btcec.PublicKey
}

func (tags *TagsInfo) UnmarshalJSON(b []byte) error {

	var arrayToCheck [][]string

	err := json.Unmarshal(b, &arrayToCheck)

	if err != nil {
		return fmt.Errorf("json.Unmarshal(b, &arrayToCheck): %+v", err)
	}

	for _, tag := range arrayToCheck {

		if len(tag) < 2 {
			return errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
		}

		tagName, err := TagFromString(tag[0])

		if err != nil {
			return errors.New(fmt.Sprintf("%s: %s", ErrInvalidTagName, tag[0]))
		}

		tagInfo := tag[1:]
		switch tagName {

		case Sigflag:
			if len(tagInfo) != 1 {
				return errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			sigFlag, err := SigFlagFromString(tagInfo[0])
			if err != nil {
				return errors.New(fmt.Sprintf("%s: %s", ErrInvalidSigFlag, tagInfo[0]))
			}

			tags.Sigflag = sigFlag

		case Pubkeys, Refund:
			if len(tagInfo) < 1 {
				return errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			for _, pubkey := range tagInfo {
				bytesPubkey, err := hex.DecodeString(pubkey)
				if err != nil {
					return fmt.Errorf("hex.DecodeString: %s", pubkey)
				}

				parsedPubkey, err := btcec.ParsePubKey(bytesPubkey)
				if err != nil {
					return fmt.Errorf("secp256k1.ParsePubKey: %s", err)
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
				return errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			nSigs, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return errors.New(fmt.Sprintf("strconv.Atoi: %s", tagInfo[0]))
			}

			tags.NSigs = nSigs

		case Locktime:
			if len(tagInfo) != 1 {
				return errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			locktime, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return errors.New(fmt.Sprintf("strconv.Atoi: %s", tagInfo[0]))
			}

			tags.Locktime = locktime
		}

	}

	return nil

}

type SpendConditionData struct {
	Nonce string
	Data  *btcec.PublicKey
	Tags  TagsInfo
}

func (scd *SpendConditionData) UnmarshalJSON(b []byte) error {

	var info = struct {
		Nonce string
		Data  string
		Tags  TagsInfo
	}{}

	err := json.Unmarshal(b, &info)

	if err != nil {
		return fmt.Errorf("json.Unmarshal(b, &info): %+v", err)
	}

	pubkey, err := hex.DecodeString(info.Data)
	if err != nil {
		return fmt.Errorf("hex.DecodeString: %s", info.Data)
	}

	parsedPubkey, err := btcec.ParsePubKey(pubkey)
	if err != nil {
		return fmt.Errorf("secp256k1.ParsePubKey: %s", err)
	}

	scd.Data = parsedPubkey
	scd.Tags = info.Tags
	scd.Nonce = info.Nonce

	return nil

}

func (sc *SpendCondition) VerifySignatures(witness *P2PKWitness, message string) (bool, error) {

	currentTime := time.Now().Unix()

	hashMessage := sha256.Sum256([]byte(message))

	// check if locktime has passed and if there are refund keys
	if sc.Data.Tags.Locktime != 0 && currentTime > int64(sc.Data.Tags.Locktime) && len(sc.Data.Tags.Refund) > 0 {
		for _, sig := range witness.Signatures {
			for _, pubkey := range sc.Data.Tags.Refund {
				if sig.Verify(hashMessage[:], pubkey) {
					return true, nil
				}
			}
		}
	}

	// append all posibles keys for signing
	amountValidSigs := 0
	signaturesToTry := append(sc.Data.Tags.Pubkeys, sc.Data.Data)

	for _, sig := range witness.Signatures {
		for _, pubkey := range signaturesToTry {
			if sig.Verify(hashMessage[:], pubkey) {
				amountValidSigs += 1
			}
		}
	}

	// check if there is a multisig set up if not check if there is only one valid signature
	switch {
	case sc.Data.Tags.NSigs > 0 && amountValidSigs >= sc.Data.Tags.NSigs:
		return true, nil

	case amountValidSigs >= 1:
		return true, nil

	default:
		return false, nil

	}
}

type Tags int

const (
	Sigflag  Tags = iota + 1
	Pubkeys  Tags = iota + 2
	NSigs    Tags = iota + 3
	Locktime Tags = iota + 4
	Refund   Tags = iota + 5
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
	default:
		return 0, errors.New(ErrInvalidTagName)
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
		return 0, errors.New(fmt.Sprintf("%s: %s", ErrInvalidTagValue, s))
	}
}

type P2PKWitness struct {
	Signatures []*schnorr.Signature
}

func (wit *P2PKWitness) UnmarshalJSON(b []byte) error {

	var sigs = struct {
		Signatures []string
	}{}

	err := json.Unmarshal(b, &sigs)

	if err != nil {
		return fmt.Errorf("json.Unmarshal(b, &info): %+v", err)
	}

	witness := P2PKWitness{
		Signatures: make([]*schnorr.Signature, 0),
	}

	for _, sig := range sigs.Signatures {
		sigBytes, err := hex.DecodeString(sig)
		if err != nil {
			return fmt.Errorf("hex.DecodeString: %s", sigBytes)
		}
		signature, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			return fmt.Errorf("schnorr.ParseSignature(sigBytes): %s", err)
		}

		witness.Signatures = append(witness.Signatures, signature)

	}

	*wit = witness

	return nil

}
