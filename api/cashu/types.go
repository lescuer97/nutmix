package cashu

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"math"
	"time"
)

var (
	ErrCouldNotParseUnitString = errors.New("Could not parse unit string")
	ErrCouldNotEncryptSeed     = errors.New("Could not encrypt seed")
	ErrCouldNotDecryptSeed     = errors.New("Could not decrypt seed")
	ErrKeysetNotFound          = errors.New("Keyset not found")
	ErrKeysetForProofNotFound  = errors.New("Keyset for proof not found")

	AlreadyActiveProof     = errors.New("Proof already being spent")
	AlreadyActiveQuote     = errors.New("Quote already being spent")
	UsingInactiveKeyset    = errors.New("Trying to use an inactive keyset")
	ErrInvalidProof        = errors.New("Invalid proof")
	ErrQuoteNotPaid        = errors.New("Quote not paid")
	ErrMessageAmountToBig  = errors.New("Message amount is to big")
	ErrInvalidBlindMessage = errors.New("Invalid blind message")

	ErrCouldNotConvertUnit         = errors.New("Could not convert unit")
	ErrCouldNotParseAmountToString = errors.New("Could not parse amount to string")

	ErrUnbalanced   = errors.New("Unbalanced transactions")
	ErrNotSameUnits = errors.New("Not same units")
)

const (
	MethodBolt11 = "bolt11"
)

const ExpiryMinutesDefault int64 = 15

func ExpiryTimeMinUnit(minutes int64) int64 {
	duration := time.Duration(minutes) * time.Minute
	return time.Now().Add(duration).Unix()
}

type Unit int

const Sat Unit = iota + 1
const Msat Unit = iota + 2
const USD Unit = iota + 3
const EUR Unit = iota + 4
const AUTH Unit = iota + 5

// String - Creating common behavior - give the type a String function
func (d Unit) String() string {
	return [...]string{"sat", "msat", "usd", "eur", "auth"}[d-1]
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
	case "usd":
		return USD, nil
	case "eur":
		return EUR, nil
	case "auth":
		return AUTH, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrCouldNotParseUnitString, s)
	}
}

var AvailableSeeds []Unit = []Unit{Sat}

type BlindedMessage struct {
	Amount  uint64 `json:"amount"`
	Id      string `json:"id"`
	B_      string `json:"B_"`
	Witness string `json:"witness,omitempty" db:"witness"`
}

func (b BlindedMessage) VerifyBlindMessageSignature(pubkeys map[*btcec.PublicKey]bool) error {
	if b.Witness == "" {
		return ErrEmptyWitness
	}
	var p2pkWitness Witness

	err := json.Unmarshal([]byte(b.Witness), &p2pkWitness)

	if err != nil {
		return fmt.Errorf("json.Unmarshal([]byte(b.Witness), &p2pkWitness)  %w", err)
	}

	decodedBlindFactor, err := hex.DecodeString(b.B_)

	if err != nil {
		return fmt.Errorf("hex.DecodeString(b.B_)  %w", err)
	}

	hash := sha256.Sum256(decodedBlindFactor)

	for _, sig := range p2pkWitness.Signatures {
		for pubkey := range pubkeys {

			ok := sig.Verify(hash[:], pubkey)
			if !ok {
				return nil
			}
		}
	}

	return nil
}

func (b BlindedMessage) GenerateBlindSignature(k *secp256k1.PrivateKey) (BlindSignature, error) {
	decodedBlindFactor, err := hex.DecodeString(b.B_)

	if err != nil {
		return BlindSignature{}, fmt.Errorf("DecodeString: %w", err)
	}

	B_, err := secp256k1.ParsePubKey(decodedBlindFactor)

	if err != nil {
		return BlindSignature{}, fmt.Errorf("ParsePubKey: %w", err)
	}

	C_ := crypto.SignBlindedMessage(B_, k)

	blindSig := BlindSignature{
		Amount: b.Amount,
		Id:     b.Id,
		C_:     hex.EncodeToString(C_.SerializeCompressed()),
	}

	err = blindSig.GenerateDLEQ(B_, k)

	if err != nil {
		return blindSig, fmt.Errorf("blindSig.GenerateDLEQ: %w", err)
	}

	return blindSig, nil
}

type BlindSignature struct {
	Amount uint64              `json:"amount"`
	Id     string              `json:"id"`
	C_     string              `json:"C_"`
	Dleq   *BlindSignatureDLEQ `json:"dleq,omitempty"`
}

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

func (p Proof) VerifyWitness(spendCondition *SpendCondition, witness *Witness, pubkeysFromProofs *map[*btcec.PublicKey]bool) (bool, error) {

	if spendCondition.Type == HTLC {
		err := spendCondition.VerifyPreimage(witness)
		if err != nil {
			return false, fmt.Errorf("spendCondition.VerifyPreimage  %w ", err)
		}
	}

	ok, pubkeys, err := spendCondition.VerifySignatures(witness, p.Secret)

	if err != nil {
		return false, fmt.Errorf("spendCondition.VerifySignatures  %w ", err)
	}

	for _, pubkey := range pubkeys {
		(*pubkeysFromProofs)[pubkey] = true
	}

	return ok, nil

}

func (p Proof) parseWitnessAndSecret() (*SpendCondition, *Witness, error) {

	var spendCondition SpendCondition
	var witness Witness

	err := json.Unmarshal([]byte(p.Secret), &spendCondition)

	if err != nil {
		return nil, nil, fmt.Errorf("json.Unmarshal([]byte(p.Secret), &spendCondition)  %w, %w", ErrCouldNotParseSpendCondition, err)

	}

	err = json.Unmarshal([]byte(p.Witness), &witness)

	if err != nil {
		return nil, nil, fmt.Errorf("json.Unmarshal([]byte(p.Witness), &witness)  %w, %w", ErrCouldNotParseWitness, err)

	}

	return &spendCondition, &witness, nil
}

func (p Proof) IsProofSpendConditioned(checkOutputs *bool) (bool, *SpendCondition, *Witness, error) {
	var witness Witness
	witnessErr := json.Unmarshal([]byte(p.Witness), &witness)

	var spendCondition SpendCondition

	spendConditionErr := json.Unmarshal([]byte(p.Secret), &spendCondition)

	switch {
	case witnessErr == nil && spendConditionErr == nil:
		// if sigflag is SigAll, then we need to check the outputs
		if spendCondition.Data.Tags.Sigflag == SigAll {
			*checkOutputs = true
		}
		return true, &spendCondition, &witness, nil
	case witnessErr != nil && spendConditionErr == nil:
		return true, nil, nil, fmt.Errorf("json.Unmarshal([]byte)  %w, %w", ErrCouldNotParseWitness, witnessErr)
	case spendConditionErr != nil && witnessErr == nil:
		return true, nil, nil, fmt.Errorf("json.Unmarshal([]byte)  %w, %w", ErrCouldNotParseSpendCondition, spendConditionErr)
	default:
		return false, nil, nil, nil
	}
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

type MintError struct {
	Detail string `json:"detail"`
	Code   int8   `json:"code"`
}

type MintKeysMap map[uint64]MintKey
type MintKey struct {
	Id          string                `json:"id"`
	Active      bool                  `json:"active" db:"active"`
	Unit        string                `json:"unit"`
	Amount      uint64                `json:"amount"`
	PrivKey     *secp256k1.PrivateKey `json:"priv_key"`
	CreatedAt   int64                 `json:"created_at"`
	InputFeePpk uint                  `json:"input_fee_ppk"`
}

func (keyset *MintKey) GetPubKey() *secp256k1.PublicKey {
	pubkey := keyset.PrivKey.PubKey()
	return pubkey
}

type Seed struct {
	Active      bool
	CreatedAt   int64
	Version     int
	Unit        string
	Id          string
	InputFeePpk uint `json:"input_fee_ppk" db:"input_fee_ppk"`
}

type SwapMintMethod struct {
	Method    string             `json:"method"`
	Unit      string             `json:"unit"`
	MinAmount int                `json:"min_amount"`
	MaxAmount int                `json:"max_amount"`
	Mpp       bool               `json:"mpp,omitempty"`
	Commands  []SubscriptionKind `json:"commands,omitempty"`
}

type SwapMintInfo struct {
	Methods   *[]SwapMintMethod `json:"methods,omitempty"`
	Disabled  *bool             `json:"disabled,omitempty"`
	Supported *bool             `json:"supported,omitempty"`
}

type ContactInfo struct {
	Method string `json:"method"`
	Info   string `json:"info"`
}

type GetInfoResponse struct {
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	Pubkey          string         `json:"pubkey"`
	Description     string         `json:"description"`
	DescriptionLong string         `json:"description_long"`
	Contact         []ContactInfo  `json:"contact"`
	Motd            string         `json:"motd"`
	Nuts            map[string]any `json:"nuts"`
}

type KeysResponse map[string][]Keyset

type Keyset struct {
	Id          string            `json:"id"`
	Unit        string            `json:"unit"`
	Keys        map[string]string `json:"keys"`
	InputFeePpk uint              `json:"input_fee_ppk"`
}

type PostMintQuoteBolt11Request struct {
	Amount int64  `json:"amount"`
	Unit   string `json:"unit"`
}

type PostMintQuoteBolt11Response struct {
	Quote   string `json:"quote"`
	Request string `json:"request"`
	// Deprecated: Should be removed after all main wallets change to the new State format
	RequestPaid bool         `json:"paid" db:"request_paid"`
	Expiry      int64        `json:"expiry"`
	Unit        string       `json:"unit"`
	Minted      bool         `json:"minted"`
	State       ACTION_STATE `json:"state"`
}
type MintRequestDB struct {
	Quote   string `json:"quote"`
	Request string `json:"request"`
	// Deprecated: Should be removed after all main wallets change to the new State format
	RequestPaid bool         `json:"paid" db:"request_paid"`
	Expiry      int64        `json:"expiry"`
	Unit        string       `json:"unit"`
	Minted      bool         `json:"minted"`
	State       ACTION_STATE `json:"state"`
	SeenAt      int64        `json:"seen_at"`
}

func (m *MintRequestDB) PostMintQuoteBolt11Response() PostMintQuoteBolt11Response {
	res := PostMintQuoteBolt11Response{
		Quote:       m.Quote,
		Request:     m.Request,
		RequestPaid: m.RequestPaid,
		Expiry:      m.Expiry,
		Unit:        m.Unit,
		Minted:      m.Minted,
		State:       m.State,
	}
	return res
}

type PostMintBolt11Request struct {
	Quote   string           `json:"quote"`
	Outputs []BlindedMessage `json:"outputs"`
}

type PostMintBolt11Response struct {
	Signatures []BlindSignature `json:"signatures"`
}

type BasicKeysetResponse struct {
	Id          string `json:"id"`
	Unit        string `json:"unit"`
	Active      bool   `json:"active"`
	InputFeePpk uint   `json:"input_fee_ppk"`
}

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
	PaymentPreimage string           `json:"payment_preimage"`
}

type PostSwapRequest struct {
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

type PostSwapResponse struct {
	Signatures []BlindSignature `json:"signatures"`
}

type PostMeltBolt11Request struct {
	Quote   string           `json:"quote"`
	Inputs  Proofs           `json:"inputs"`
	Outputs []BlindedMessage `json:"outputs"`
}

type PostCheckStateRequest struct {
	Ys []string `json:"Ys"`
}

type CheckState struct {
	Y       string     `json:"Y"`
	State   ProofState `json:"state"`
	Witness *string    `json:"witness,omitempty"`
}

type PostCheckStateResponse struct {
	States []CheckState `json:"states"`
}

type RecoverSigDB struct {
	Amount    uint64              `json:"amount"`
	Id        string              `json:"id"`
	B_        string              `json:"B_" db:"B_"`
	C_        string              `json:"C_" db:"C_"`
	CreatedAt int64               `json:"created_at" db:"created_at"`
	Dleq      *BlindSignatureDLEQ `json:"dleq,omitempty"`

	// This fields are use for melt_requests pending queries
	MeltQuote string `json:"melt_quote" db:"melt_quote"`
}

func (r RecoverSigDB) GetSigAndMessage() (BlindSignature, BlindedMessage) {
	return r.GetBlindSignature(), r.GetBlindedMessage()
}
func (r RecoverSigDB) GetBlindedMessage() BlindedMessage {
	return BlindedMessage{
		Amount: r.Amount,
		Id:     r.Id,
		B_:     r.B_,
	}
}

func (r RecoverSigDB) GetBlindSignature() BlindSignature {
	return BlindSignature{
		Amount: r.Amount,
		Id:     r.Id,
		C_:     r.C_,
		Dleq:   r.Dleq,
	}
}

type PostRestoreRequest struct {
	Outputs []BlindedMessage `json:"outputs"`
}

type PostRestoreResponse struct {
	Outputs    []BlindedMessage `json:"outputs"`
	Signatures []BlindSignature `json:"signatures"`
	Promises   []BlindSignature `json:"promises"`
}

type BlindSignatureDLEQ struct {
	E *secp256k1.PrivateKey `json:"e"`
	S *secp256k1.PrivateKey `json:"s"`
}

func (b *BlindSignatureDLEQ) UnmarshalJSON(data []byte) error {
	var aux struct {
		E string `json:"e"`
		S string `json:"s"`
	}

	err := json.Unmarshal(data, &aux)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	e_bytes, err := hex.DecodeString(aux.E)
	if err != nil {
		return fmt.Errorf("hex.DecodeString(aux.E): %w", err)
	}

	s_bytes, err := hex.DecodeString(aux.S)
	if err != nil {
		return fmt.Errorf("hex.DecodeString(aux.S): %w", err)
	}

	e := secp256k1.PrivKeyFromBytes(e_bytes)
	s := secp256k1.PrivKeyFromBytes(s_bytes)

	b.E = e
	b.S = s

	return nil
}

func (b *BlindSignatureDLEQ) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("null"), nil
	}
	if b.E == nil || b.S == nil {
		return []byte("null"), nil
	}

	return json.Marshal(&struct {
		E string `json:"e"` // We want to encode the E as a string
		S string `json:"s"` // We want to encode the S as a string
	}{
		E: b.E.Key.String(),
		S: b.S.Key.String(),
	})
}

func (b *BlindSignature) GenerateDLEQ(B_ *secp256k1.PublicKey, a *secp256k1.PrivateKey) error {
	// Generate nonce private key
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		return fmt.Errorf("rand.Read: %w", err)
	}
	r := secp256k1.PrivKeyFromBytes(nonce)

	// R1 = r * G
	R1 := r.PubKey()

	// R2 = r * B_
	var blindMessagePoint, R2 secp256k1.JacobianPoint
	B_.AsJacobian(&blindMessagePoint)

	btcec.ScalarMultNonConst(&r.Key, &blindMessagePoint, &R2)
	R2.ToAffine()

	// Convert C_ String to secp256k1.PublicKey
	C_bytes, err := hex.DecodeString(b.C_)
	if err != nil {
		return fmt.Errorf("hex.DecodeString(b.C_): %w", err)
	}

	C_, err := secp256k1.ParsePubKey(C_bytes)

	if err != nil {
		return fmt.Errorf("secp256k1.ParsePubKey: %w", err)
	}

	// generate e = hash(R1,R2,A,C')
	keys := []*secp256k1.PublicKey{R1, btcec.NewPublicKey(&R2.X, &R2.Y), a.PubKey(), C_}

	ehash := crypto.Hash_e(keys)

	e := secp256k1.PrivKeyFromBytes(ehash[:])

	// generate s = r + e*a

	// e*a
	e.Key.Mul(&a.Key)

	// r * ea
	s := secp256k1.NewPrivateKey(r.Key.Add(&e.Key))

	// I don't use e here because the original variable got altered when multiplying for a.Key
	b.Dleq = &BlindSignatureDLEQ{E: secp256k1.PrivKeyFromBytes(ehash[:]), S: s}

	return nil
}

// R1 = s*G - e*A
//
// R2 = s*B' - e*C'
// e == hash(R1,R2,A,C')
//
// If true, a in A = a*G must be equal to a in C' = a*B'
func (b *BlindSignature) VerifyDLEQ(
	blindMessage *secp256k1.PublicKey,
	e *secp256k1.PrivateKey,
	s *secp256k1.PrivateKey,
	A *secp256k1.PublicKey,
) (bool, error) {
	// e * A
	var a_point, eA secp256k1.JacobianPoint

	// negate the e key
	e.Key.Negate()
	A.AsJacobian(&a_point)

	// (-e) * A
	btcec.ScalarMultNonConst(&e.Key, &a_point, &eA)
	eA.ToAffine()

	// s*G
	var sG, R1 secp256k1.JacobianPoint

	sPubKey := s.PubKey()
	sPubKey.AsJacobian(&sG)

	// s*G + ((-e)*A)
	btcec.AddNonConst(&sG, &eA, &R1)
	R1.ToAffine()

	var eC, c_point secp256k1.JacobianPoint

	// Parse BlindSignature to Pubkey
	C_bytes, err := hex.DecodeString(b.C_)
	if err != nil {
		return false, fmt.Errorf("hex.DecodeString(b.C_): %w", err)
	}

	C_, err := secp256k1.ParsePubKey(C_bytes)

	if err != nil {
		return false, fmt.Errorf("secp256k1.ParsePubKey: %w", err)
	}

	C_.AsJacobian(&c_point)

	// e*C'
	secp256k1.ScalarMultNonConst(&e.Key, &c_point, &eC)
	eC.ToAffine()

	// s*B
	var R2, sB, point_b secp256k1.JacobianPoint

	blindMessage.AsJacobian(&point_b)
	btcec.ScalarMultNonConst(&s.Key, &point_b, &sB)
	sB.ToAffine()

	// R2 = s*B' + ((-e)*C')
	btcec.AddNonConst(&sB, &eC, &R2)
	R2.ToAffine()

	keys := []*secp256k1.PublicKey{secp256k1.NewPublicKey(&R1.X, &R1.Y), secp256k1.NewPublicKey(&R2.X, &R2.Y), A, C_}

	hashed_keys := crypto.Hash_e(keys)

	hashed_keys_priv := secp256k1.PrivKeyFromBytes(hashed_keys[:])

	// I negate the hashed_keys_priv because the original key got altered when multiplying for A
	return hashed_keys_priv.Key.Negate().String() == e.Key.String(), nil

}

type MeltChange struct {
	B_        string `db:"B_"`
	Id        string `db:"id"`
	Quote     string `db:"quote"`
	CreatedAt int64  `json:"created_at" db:"created_at"`
}

type Amount struct {
	Unit   Unit
	Amount uint64
}

func (a *Amount) To(toUnit Unit) error {
	if a.Unit == toUnit {
		return nil
	}
	switch toUnit {
	case Msat:
		if a.Unit == Sat {
			a.Unit = toUnit
			a.Amount = a.Amount * 1000
			return nil
		}
	case Sat:
		if a.Unit == Msat {
			a.Unit = toUnit
			amount := float64(a.Amount) / 1000
			amount = math.Floor(amount)
			a.Amount = uint64(amount)
			return nil
		}
	default:
		return ErrCouldNotConvertUnit
	}
	return nil
}
func (a *Amount) ToFloatString() (string, error) {
	if a.Unit == USD || a.Unit == EUR {
		return a.CentsToUSD()
	} else if a.Unit == Sat {
		return a.SatToBTC()
	} else {
		return "", fmt.Errorf("Amount must be in satoshis or cents")
	}
}

func (a *Amount) SatToBTC() (string, error) {
	if a.Unit != Sat {
		return "", ErrCouldNotParseSpendCondition
	}
	btc := float64(a.Amount) / 1e8
	return fmt.Sprintf("%.8f", btc), nil
}

func (a *Amount) CentsToUSD() (string, error) {
	if a.Unit != USD && a.Unit != EUR {
		return "", ErrCouldNotParseSpendCondition
	}
	dollars := float64(a.Amount) / 100
	return fmt.Sprintf("%.2f", dollars), nil
}
