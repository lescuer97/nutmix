package cashu

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"strconv"
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

type TagsInfo map[Tags][]string

func (tags *TagsInfo) UnmarshalJSON(b []byte) error {

	tagsInfo, err := GetTagsInfo(b)

	if err != nil {
		return err
	}

	*tags = tagsInfo
	return nil
}

type SpendConditionData struct {
	Nonce string
	Data  string
	Tags  TagsInfo
}

// func (scp *SpendConditionData) UnmarshalJSON(b []byte) error {
// 	// a := []interface{}{&scp.Nonce, &scp.Data /* , &scp.Tags */ }
// 	a := []interface{}{&scp.Nonce, &scp.Data  , &scp.Tags  }
// 	return json.Unmarshal(b, &a)
// }

func GetTagsInfo(b []byte) (TagsInfo, error) {

	tagsInfo := make(TagsInfo)

	var arrayToCheck [][]string

	fmt.Printf("b %+v", string(b))
	err := json.Unmarshal(b, &arrayToCheck)

	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal(b, &arrayToCheck): %+v", err)

	}

	fmt.Printf("arrayToCheck %+v: ", arrayToCheck)

	for _, tag := range arrayToCheck {

		if len(tag) < 2 {
			return nil, errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
		}

		tagName, err := TagFromString(tag[0])

		if err != nil {
			return nil, errors.New(fmt.Sprintf("%s: %s", ErrInvalidTagName, tag[0]))
		}

		tagInfo := tag[1:]
		switch tagName {

		case Sigflag:
			if len(tagInfo) != 1 {
				return nil, errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			sigFlag, err := SigFlagFromString(tagInfo[0])
			if err != nil {
				return nil, errors.New(fmt.Sprintf("%s: %s", ErrInvalidSigFlag, tagInfo[0]))
			}
			tagsInfo[tagName] = append(tagsInfo[tagName], sigFlag.String())

		case Pubkeys, Refund:
			if len(tagInfo) < 1 {
				return nil, errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			for _, pubkey := range tagInfo {
				bytesPubkey, err := hex.DecodeString(pubkey)
				if err != nil {
					return nil, fmt.Errorf("hex.DecodeString: %s", pubkey)
				}

				parsedPubkey, err := secp256k1.ParsePubKey(bytesPubkey)
				if err != nil {
					return nil, fmt.Errorf("secp256k1.ParsePubKey: %s", err)
				}

				tagsInfo[tagName] = append(tagsInfo[tagName], hex.EncodeToString(parsedPubkey.SerializeCompressed()))

			}

		case NSigs:
			if len(tagInfo) != 1 {
				return nil, errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			nSigs, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return nil, errors.New(fmt.Sprintf("strconv.Atoi: %s", tagInfo[0]))
			}

			tagsInfo[tagName] = append(tagsInfo[tagName], strconv.Itoa(nSigs))

		case Locktime:
			if len(tagInfo) != 1 {
				return nil, errors.New(fmt.Sprintf("%s: %s", ErrMalformedTag, tag))
			}

			locktime, err := strconv.Atoi(tagInfo[0])
			if err != nil {
				return nil, errors.New(fmt.Sprintf("strconv.Atoi: %s", tagInfo[0]))
			}
			tagsInfo[tagName] = append(tagsInfo[tagName], strconv.Itoa(locktime))
		}

	}

	fmt.Printf("tagsInfo %+v: ", tagsInfo)

	return tagsInfo, nil

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
