package cashu

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"database/sql/driver"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
)

var (
	ErrCouldNotParseUnitString = errors.New("could not parse unit string")
	ErrCouldNotParsePublicKey  = errors.New("could not parse public key string")
	ErrCouldNotEncryptSeed     = errors.New("could not encrypt seed")
	ErrCouldNotDecryptSeed     = errors.New("could not decrypt seed")
	ErrKeysetNotFound          = errors.New("keyset not found")
	ErrKeysetForProofNotFound  = errors.New("keyset for proof not found")

	ErrAlreadyActiveProof          = errors.New("proof already being spent")
	ErrAlreadyActiveQuote          = errors.New("quote already being spent")
	ErrUsingInactiveKeyset         = errors.New("trying to use an inactive keyset")
	ErrInvalidProof                = errors.New("invalid proof")
	ErrQuoteNotPaid                = errors.New("quote not paid")
	ErrMessageAmountToBig          = errors.New("message amount is to big")
	ErrInvalidBlindMessage         = errors.New("invalid blind message")
	ErrInvalidBlindSignature       = errors.New("invalid blind signature")
	ErrCouldNotConvertUnit         = errors.New("could not convert unit")
	ErrCouldNotParseAmountToString = errors.New("could not parse amount to string")
	ErrUnbalanced                  = errors.New("unbalanced transactions")
	ErrNotSameUnits                = errors.New("not same units")
	ErrRepeatedOutput              = errors.New("duplicate outputs provided")
	ErrRepeatedInput               = errors.New("duplicate inputs provided")
	ErrPaymentFailed               = errors.New("lightning payment failed")
	ErrPaymentNoRoute              = errors.New("no route found for lightning payment")

	ErrMintQuoteNoPublicKey      = errors.New("no valid pubkey for mint quote")
	ErrMintQuoteNoValidSignature = errors.New("no valid signature for mint quote")
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
	B_      WrappedPublicKey `json:"B_"`
	Id      string           `json:"id"`
	Witness string           `json:"witness,omitempty" db:"witness"`
	Amount  uint64           `json:"amount"`
}

func (b BlindedMessage) GenerateBlindSignature(k *secp256k1.PrivateKey) (BlindSignature, error) {

	C_ := crypto.SignBlindedMessage(b.B_.PublicKey, k)

	blindSig := BlindSignature{
		Amount: b.Amount,
		Id:     b.Id,
		C_:     WrappedPublicKey{PublicKey: C_},
	}

	err := blindSig.GenerateDLEQ(b.B_.PublicKey, k)
	if err != nil {
		return blindSig, fmt.Errorf("blindSig.GenerateDLEQ: %w", err)
	}

	return blindSig, nil
}

type BlindSignature struct {
	C_     WrappedPublicKey    `json:"C_"`
	Dleq   *BlindSignatureDLEQ `json:"dleq,omitempty"`
	Id     string              `json:"id"`
	Amount uint64              `json:"amount"`
}

type MintError struct {
	Detail string `json:"detail"`
	Code   int8   `json:"code"`
}

type MintKeysMap map[uint64]MintKey
type MintKey struct {
	PrivKey     *secp256k1.PrivateKey `json:"priv_key"`
	FinalExpiry *uint64               `json:"final_expiry"`
	Id          string                `json:"id"`
	Unit        string                `json:"unit"`
	Amount      uint64                `json:"amount"`
	CreatedAt   int64                 `json:"created_at"`
	InputFeePpk uint                  `json:"input_fee_ppk"`
	Active      bool                  `json:"active" db:"active"`
}

func (keyset *MintKey) GetPubKey() *secp256k1.PublicKey {
	pubkey := keyset.PrivKey.PubKey()
	return pubkey
}

func OrderedListOfPubkeys(listKeys []MintKey) []*secp256k1.PublicKey {
	sort.Slice(listKeys, func(i, j int) bool {
		return listKeys[i].Amount < listKeys[j].Amount
	})

	pubkeys := make([]*secp256k1.PublicKey, 0)
	for i := range listKeys {
		if listKeys[i].PrivKey == nil {
			panic("Private key should have never been null at this point")
		}
		pubkeys = append(pubkeys, listKeys[i].PrivKey.PubKey())
	}
	return pubkeys
}

type Seed struct {
	FinalExpiry    *uint64 `json:"final_expiry" db:"final_expiry"`
	Unit           string
	Id             string
	DerivationPath string   `json:"derivation_path" db:"derivation_path"`
	Amounts        []uint64 `json:"amounts" db:"amounts"`
	CreatedAt      int64
	InputFeePpk    uint `json:"input_fee_ppk" db:"input_fee_ppk"`
	Version        uint32
	Active         bool
	Legacy         bool `json:"legacy" db:"legacy"`
}

type SwapMintMethod struct {
	Options   *SwapMintMethodOptions `json:"options,omitempty"`
	Method    string                 `json:"method"`
	Unit      string                 `json:"unit"`
	Commands  []SubscriptionKind     `json:"commands,omitempty"`
	MinAmount int                    `json:"min_amount,omitempty"`
	MaxAmount int                    `json:"max_amount,omitempty"`
}
type SwapMintMethodOptions struct {
	Description *bool `json:"description,omitempty"`
}

type MultipathPaymentSetting struct {
	Method string `json:"method"`
	Unit   string `json:"unit"`
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
	Nuts            map[string]any `json:"nuts"`
	TosUrl          *string        `json:"tos_url,omitempty"`
	IconUrl         *string        `json:"icon_url,omitempty"`
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	Pubkey          string         `json:"pubkey"`
	Description     string         `json:"description"`
	DescriptionLong string         `json:"description_long"`
	Motd            string         `json:"motd"`
	Contact         []ContactInfo  `json:"contact"`
	Time            int64          `json:"time"`
}

type KeysResponse map[string][]Keyset

type Keyset struct {
	Keys        map[string]string `json:"keys"`
	FinalExpiry *uint64           `json:"final_expiry,omitempty" db:"final_expiry"`
	Id          string            `json:"id"`
	Unit        string            `json:"unit"`
	InputFeePpk uint              `json:"input_fee_ppk"`
}

type PostMintQuoteBolt11Request struct {
	Description *string          `json:"description,omitempty"`
	Pubkey      WrappedPublicKey `json:"pubkey"`
	Unit        string           `json:"unit"`
	Amount      uint64           `json:"amount"`
}

type PostMintQuoteBolt11Response struct {
	Amount  *uint64          `json:"amount,omitempty"`
	Pubkey  WrappedPublicKey `json:"pubkey"`
	Quote   string           `json:"quote"`
	Request string           `json:"request"`
	Unit    string           `json:"unit"`
	State   ACTION_STATE     `json:"state"`
	Expiry  int64            `json:"expiry"`
	Minted  bool             `json:"minted"`
}

func (r PostMintQuoteBolt11Response) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Pubkey  *string      `json:"pubkey,omitempty"`
		Amount  *uint64      `json:"amount,omitempty"`
		Quote   string       `json:"quote"`
		Request string       `json:"request"`
		Unit    string       `json:"unit"`
		State   ACTION_STATE `json:"state"`
		Expiry  int64        `json:"expiry"`
		Minted  bool         `json:"minted"`
	}

	var alias Alias
	alias.Quote = r.Quote
	alias.Request = r.Request
	alias.Expiry = r.Expiry
	alias.Unit = r.Unit
	alias.Minted = r.Minted
	alias.State = r.State
	alias.Amount = r.Amount
	pubkeyStr := r.Pubkey.ToHex()
	alias.Pubkey = &pubkeyStr

	return json.Marshal(&alias)
}

type MintRequestDB struct {
	Amount      *uint64          `json:"amount"`
	Pubkey      WrappedPublicKey `json:"pubkey"`
	Description *string          `json:"description,omitempty"`
	Quote       string           `json:"quote"`
	Request     string           `json:"request"`
	Unit        string           `json:"unit"`
	State       ACTION_STATE     `json:"state"`
	CheckingId  string           `json:"checking_id"`
	Expiry      int64            `json:"expiry"`
	SeenAt      int64            `json:"seen_at"`
	Minted      bool             `json:"minted"`
}

func (m *MintRequestDB) PostMintQuoteBolt11Response() PostMintQuoteBolt11Response {
	res := PostMintQuoteBolt11Response{
		Quote:   m.Quote,
		Request: m.Request,
		Expiry:  m.Expiry,
		Unit:    m.Unit,
		Minted:  m.Minted,
		State:   m.State,
		Pubkey:  m.Pubkey,
	}

	if m.Amount != nil {
		res.Amount = m.Amount

	}
	return res
}

type PostMintBolt11Request struct {
	Signature *schnorr.Signature `json:"signature,omitempty"`
	Quote     string             `json:"quote"`
	Outputs   []BlindedMessage   `json:"outputs"`
}

func (p *PostMintBolt11Request) UnmarshalJSON(data []byte) error {
	var aux struct {
		Signature *string          `json:"signature,omitempty"`
		Quote     string           `json:"quote"`
		Outputs   []BlindedMessage `json:"outputs"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("could not marshall into PostMintBolt11Request: %w", err)
	}

	p.Outputs = aux.Outputs
	p.Quote = aux.Quote

	if aux.Signature != nil && *aux.Signature != "" {
		sig, err := parseBIP340SignatureString(*aux.Signature)
		if err != nil {
			return fmt.Errorf("failed to parse signature: %w", err)
		}

		p.Signature = sig
	} else {
		p.Signature = nil
	}

	return nil
}
func (p *PostMintBolt11Request) VerifyPubkey(pubkey *secp256k1.PublicKey) (bool, error) {
	if pubkey == nil {
		return false, fmt.Errorf("pubkey is nil. %w", ErrMintQuoteNoPublicKey)
	}

	if p.Signature == nil {
		return false, fmt.Errorf("signature not available for verification. %w", ErrMintQuoteNoValidSignature)
	}

	var msg strings.Builder
	msg.WriteString(p.Quote)
	for _, output := range p.Outputs {
		msg.WriteString(output.B_.String())
	}
	hash := sha256.Sum256([]byte(msg.String()))

	return p.Signature.Verify(hash[:], pubkey), nil
}

// parseBIP340SignatureString converts a BIP-340 Schnorr signature hex string to schnorr.Signature
func parseBIP340SignatureString(sigStr string) (*schnorr.Signature, error) {
	// Remove "0x" prefix if present
	if len(sigStr) >= 2 && sigStr[:2] == "0x" {
		sigStr = sigStr[2:]
	}

	// BIP-340 signatures are exactly 64 bytes (128 hex characters)
	if len(sigStr) != 128 {
		return nil, fmt.Errorf("invalid BIP-340 signature length: expected 128 hex characters, got %d", len(sigStr))
	}

	// Decode hex string to bytes
	sigBytes, err := hex.DecodeString(sigStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex signature: %w", err)
	}

	// Parse the signature bytes into a schnorr.Signature
	signature, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schnorr signature: %w", err)
	}

	return signature, nil
}

type PostMintBolt11Response struct {
	Signatures []BlindSignature `json:"signatures"`
}

type BasicKeysetResponse struct {
	FinalExpiry *uint64 `json:"final_expiry,omitempty"`
	Id          string  `json:"id"`
	Unit        string  `json:"unit"`
	InputFeePpk uint    `json:"input_fee_ppk"`
	Version     uint32
	Active      bool `json:"active"`
}

type PostCheckStateRequest struct {
	Ys []WrappedPublicKey `json:"Ys"`
}

type CheckState struct {
	Y       WrappedPublicKey `json:"Y"`
	Witness *string          `json:"witness,omitempty"`
	State   ProofState       `json:"state"`
}

type PostCheckStateResponse struct {
	States []CheckState `json:"states"`
}

type RecoverSigDB struct {
	B_        WrappedPublicKey    `json:"B_" db:"B_"`
	C_        WrappedPublicKey    `json:"C_" db:"C_"`
	Dleq      *BlindSignatureDLEQ `json:"dleq,omitempty"`
	Id        string              `json:"id"`
	MeltQuote string              `json:"melt_quote" db:"melt_quote"`
	Amount    uint64              `json:"amount"`
	CreatedAt int64               `json:"created_at" db:"created_at"`
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

	// generate e = hash(R1,R2,A,C')
	keys := []*secp256k1.PublicKey{R1, btcec.NewPublicKey(&R2.X, &R2.Y), a.PubKey(), b.C_.PublicKey}

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

	b.C_.AsJacobian(&c_point)

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

	keys := []*secp256k1.PublicKey{secp256k1.NewPublicKey(&R1.X, &R1.Y), secp256k1.NewPublicKey(&R2.X, &R2.Y), A, b.C_.PublicKey}

	hashed_keys := crypto.Hash_e(keys)

	hashed_keys_priv := secp256k1.PrivKeyFromBytes(hashed_keys[:])

	// I negate the hashed_keys_priv because the original key got altered when multiplying for A
	return hashed_keys_priv.Key.Negate().String() == e.Key.String(), nil

}

type MeltChange struct {
	B_        WrappedPublicKey `db:"B_"`
	Id        string           `db:"id"`
	Quote     string           `db:"quote"`
	CreatedAt int64            `json:"created_at" db:"created_at"`
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
	switch a.Unit {
	case USD, EUR:
		return a.CentsToUSD()
	case Sat:
		return a.SatToBTC()
	default:
		return "", fmt.Errorf("amount must be in satoshis or cents")
	}
}

func (a *Amount) SatToBTC() (string, error) {
	if a.Unit != Sat {
		return "", ErrCouldNotParseAmountToString
	}
	btc := float64(a.Amount) / 1e8
	return fmt.Sprintf("%.8f", btc), nil
}

func (a *Amount) CentsToUSD() (string, error) {
	if a.Unit != USD && a.Unit != EUR {
		return "", ErrCouldNotParseAmountToString
	}
	dollars := float64(a.Amount) / 100
	return fmt.Sprintf("%.2f", dollars), nil
}

type WrappedPublicKey struct {
	*secp256k1.PublicKey
}

func (p WrappedPublicKey) Value() (driver.Value, error) {
	if p.PublicKey == nil {
		return nil, nil
	}
	return p.SerializeCompressed(), nil
}

func (p *WrappedPublicKey) Scan(value any) error {
	if value == nil {
		p.PublicKey = nil
		return nil
	}

	var bytesValue []byte
	switch v := value.(type) {
	case string:
		bytesFromHex, err := hex.DecodeString(v)
		if err != nil {
			return fmt.Errorf("failed to decode hex string: %w", err)
		}
		bytesValue = bytesFromHex
	case []byte:
		bytesValue = v
	default:
		return fmt.Errorf("failed to scan PublicKey: value is not a string or []byte")
	}

	pubKey, err := btcec.ParsePubKey(bytesValue)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	p.PublicKey = pubKey
	return nil
}

// MarshalJSON implements json.Marshaler to encode the public key as a
// compressed hex string (or null).
func (p WrappedPublicKey) MarshalJSON() ([]byte, error) {
	if p.PublicKey == nil {
		return json.Marshal(nil)
	}
	s := hex.EncodeToString(p.SerializeCompressed())
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler to decode a compressed hex
// string into the underlying *secp256k1.PublicKey.
func (p *WrappedPublicKey) UnmarshalJSON(b []byte) error {
	// allow null
	if string(b) == "null" {
		p.PublicKey = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		p.PublicKey = nil
		return nil
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return errors.Join(ErrCouldNotParsePublicKey, err)
	}
	pubKey, err := btcec.ParsePubKey(decoded)
	if err != nil {
		return errors.Join(ErrCouldNotParsePublicKey, err)
	}
	p.PublicKey = pubKey
	return nil
}

// MarshalText implements encoding.TextMarshaler so this type can also be
// encoded/decoded by libraries that use text (e.g., some DB drivers).
func (p WrappedPublicKey) MarshalText() ([]byte, error) {
	if p.PublicKey == nil {
		return nil, nil
	}

	return p.SerializeCompressed(), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *WrappedPublicKey) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		p.PublicKey = nil
		return nil
	}
	pubKey, err := btcec.ParsePubKey(text)
	if err != nil {
		return errors.Join(ErrCouldNotParsePublicKey, err)
	}
	p.PublicKey = pubKey
	return nil
}

// ToHex returns the compressed-hex representation of the wrapped public key.
// It is nil-safe and returns the empty string when the underlying key is nil.
func (p WrappedPublicKey) ToHex() string {
	if p.PublicKey == nil {
		return ""
	}
	return hex.EncodeToString(p.SerializeCompressed())
}

// String implements fmt.Stringer and returns the hex representation (or empty
// string when nil).
func (p WrappedPublicKey) String() string {
	return p.ToHex()
}
