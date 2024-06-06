package cashu

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
)

var ExpiryTime int64 = time.Now().Add(15 * time.Minute).Unix()

type Unit int

const Sat Unit = iota + 1
const Msat Unit = iota + 2

// String - Creating common behavior - give the type a String function
func (d Unit) String() string {
	return [...]string{"sat", "msat"}[d-1]
}

// EnumIndex - Creating common behavior - give the type a EnumIndex functio
func (d Unit) EnumIndex() int {
	return int(d)
}

func UnitFromString(s string) (Unit, error) {
	switch s {
	case "sat":
		return Sat, nil
	case "msat":
		return Msat, nil
	default:
		return 0, fmt.Errorf("Invalid unit: %s", s)
	}
}

var AvailableSeeds []Unit = []Unit{Sat}

type BlindedMessage struct {
	Amount uint64  `json:"amount"`
	Id     string `json:"id"`
	B_     string `json:"B_"`
}

func (b BlindedMessage) GenerateBlindSignature(k *secp256k1.PrivateKey) (BlindSignature, error) {
	decodedBlindFactor, err := hex.DecodeString(b.B_)

	if err != nil {
		log.Println(fmt.Errorf("DecodeString: %v", err))
		return BlindSignature{}, err
	}

	B_, err := secp256k1.ParsePubKey(decodedBlindFactor)

	if err != nil {
		log.Println(fmt.Errorf("ParsePubKey: %v", err))
		return BlindSignature{}, err
	}

	C_ := crypto.SignBlindedMessage(B_, k)

	return BlindSignature{
		Amount: b.Amount,
		Id:     b.Id,
		C_:     hex.EncodeToString(C_.SerializeCompressed()),
	}, nil
}

type BlindSignature struct {
	Amount uint64  `json:"amount"`
	Id     string `json:"id"`
	C_     string `json:"C_"`
}

type ProofState string

const UNSPENT ProofState = "UNSPENT"
const SPENT ProofState = "SPENT"
const PENDING ProofState = "PENDING"

type Proof struct {
	Amount uint64  `json:"amount"`
	Id     string `json:"id"`
	Secret string `json:"secret"`
	C      string `json:"C" db:"c"`
	Y      string `json:"Y" db:"Y"`
}

func (p Proof) HashSecretToCurve() (Proof, error) {

	// Get Hash to curve of secret
	parsedProof := []byte(p.Secret)

	y, err := crypto.HashToCurve(parsedProof)

	if err != nil {
		log.Printf("crypto.HashToCurve: %+v", err)
		return p, err
	}

	Y_hex := hex.EncodeToString(y.SerializeCompressed())
	p.Y = Y_hex
	return p, nil
}

type MintError struct {
	Detail string `json:"detail"`
	Code   int8   `json:"code"`
}

type Keyset struct {
	Id        string                `json:"id"`
	Active    bool                  `json:"active" db:"active"`
	Unit      string                `json:"unit"`
	Amount    uint64                   `json:"amount"`
	PrivKey   *secp256k1.PrivateKey `json:"priv_key"`
	CreatedAt int64                 `json:"created_at"`
}

func (keyset *Keyset) GetPubKey() *secp256k1.PublicKey {
	pubkey := keyset.PrivKey.PubKey()
	return pubkey
}

type Seed struct {
	Seed      []byte
	Active    bool
	CreatedAt int64
	Version   int
	Unit      string
	Id        string
}

type SwapMintMethod struct {
	Method    string `json:"method"`
	Unit      string `json:"unit"`
	MinAmount int    `json:"min_amount"`
	MaxAmount int    `json:"max_amount"`
}

type SwapMintInfo struct {
	Methods  *[]SwapMintMethod `json:"methods,omitempty"`
	Disabled bool              `json:"disabled"`
}

type GetInfoResponse struct {
	Name            string     `json:"name"`
	Version         string     `json:"version"`
	Pubkey          string     `json:"pubkey"`
	Description     string     `json:"description"`
	DescriptionLong string     `json:"description_long"`
	Contact         [][]string `json:"contact"`
	Motd            string     `json:"motd"`
	Nuts            map[string]SwapMintInfo
}

type KeysResponse map[string][]KeysetResponse

type KeysetResponse struct {
	Id   string            `json:"id"`
	Unit string            `json:"unit"`
	Keys map[string]string `json:"keys"`
}

type PostMintQuoteBolt11Request struct {
	Amount int64  `json:"amount"`
	Unit   string `json:"unit"`
}

type PostMintQuoteBolt11Response struct {
	Quote   string `json:"quote"`
    Request string `json:"request"`
    RequestPaid bool `json:"paid" db:"request_paid"`
	Expiry  int64  `json:"expiry"`
    Unit   string `json:"unit"`
    Minted bool   `json:"minted"`
}

type PostMintBolt11Request struct {
	Quote   string           `json:"quote"`
	Outputs []BlindedMessage `json:"outputs"`
}

type PostMintBolt11Response struct {
	Signatures []BlindSignature `json:"signatures"`
}

type BasicKeysetResponse struct {
	Id     string `json:"id"`
	Unit   string `json:"unit"`
	Active bool   `json:"active"`
}

type MeltRequestDB struct {
	Quote      string `json:"quote"`
	Unit       string `json:"unit"`
	Expiry     int64  `json:"expiry"`
	Amount     uint64  `json:"amount"`
	FeeReserve uint64  `json:"fee_reserve" db:"fee_reserve"`
	RequestPaid       bool   `json:"paid" db:"request_paid"`
	Request    string `json:"request"`
    Melted     bool   `json:"melted"`
}

func (meltRequest *MeltRequestDB) GetPostMeltQuoteResponse() PostMeltQuoteBolt11Response {
	return PostMeltQuoteBolt11Response{
		Quote:      meltRequest.Quote,
		Amount:     meltRequest.Amount,
		FeeReserve: meltRequest.FeeReserve,
		Paid:       meltRequest.RequestPaid,
		Expiry:     meltRequest.Expiry,
	}

}

type PostMeltQuoteBolt11Request struct {
	Request string `json:"request"`
	Unit    string `json:"unit"`
}

type PostMeltQuoteBolt11Response struct {
	Quote      string `json:"quote"`
	Amount     uint64  `json:"amount"`
	FeeReserve uint64  `json:"fee_reserve"`
	Paid       bool   `json:"paid"`
	Expiry     int64  `json:"expiry"`
}

type PostSwapRequest struct {
	Inputs  []Proof          `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

type PostSwapResponse struct {
	Signatures []BlindSignature `json:"signatures"`
}

type PostMeltBolt11Request struct {
	Quote  string  `json:"quote"`
	Inputs []Proof `json:"inputs"`
}

type PostMeltBolt11Response struct {
	Paid            bool   `json:"paid"`
	PaymentPreimage string `json:"payment_preimage"`
}

type PostCheckStateRequest struct {
	Ys []string `json:"Ys"`
}

type CheckState struct {
	Y       string     `json:"Y"`
	State   ProofState `json:"state"`
	Witness *string    `json:"witness"`
}

type PostCheckStateResponse struct {
	States []CheckState `json:"states"`
}
