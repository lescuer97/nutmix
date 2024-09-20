package mint

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/pkg/crypto"
)

type Mint struct {
	ActiveKeysets    map[string]cashu.KeysetMap
	Keysets          map[string][]cashu.Keyset
	LightningBackend lightning.LightningBackend
	PendingProofs    []cashu.Proof
	ActiveProofs     *ActiveProofs
	ActiveQuotes     *ActiveQuote
	Config           Config
	MintPubkey       string
}

var (
	AlreadyActiveProof = errors.New("Proof already being spent")
	AlreadyActiveQuote = errors.New("Quote already being spent")
)

type ActiveProofs struct {
	Proofs map[cashu.Proof]bool
	sync.Mutex
}

func (a *ActiveProofs) AddProofs(proofs []cashu.Proof) error {
	a.Lock()
	defer a.Unlock()
	// check if proof already exists
	for _, p := range proofs {

		if a.Proofs[p] {
			return AlreadyActiveProof
		}

		a.Proofs[p] = true
	}
	return nil
}

func (a *ActiveProofs) RemoveProofs(proofs []cashu.Proof) error {
	a.Lock()
	defer a.Unlock()
	// check if proof already exists
	for _, p := range proofs {

		delete(a.Proofs, p)

	}
	return nil
}

type ActiveQuote struct {
	Quote map[string]bool
	sync.Mutex
}

func (q *ActiveQuote) AddQuote(quote string) error {
	q.Lock()

	defer q.Unlock()

	if q.Quote[quote] {
		return AlreadyActiveQuote
	}

	q.Quote[quote] = true

	return nil
}
func (q *ActiveQuote) RemoveQuote(quote string) error {
	q.Lock()
	defer q.Unlock()

	delete(q.Quote, quote)

	return nil
}
func (m *Mint) AddQuotesAndProofs(quote string, proofs []cashu.Proof) error {

	if quote != "" {
		err := m.ActiveQuotes.AddQuote(quote)
		if err != nil {
			return fmt.Errorf("m.AddActiveMintQuote(quote): %w", err)
		}
	}

	if len(proofs) == 0 {
		err := m.ActiveProofs.AddProofs(proofs)
		if err != nil {
			return fmt.Errorf("AddProofs: %w", err)
		}

	}
	return nil
}

func (m *Mint) RemoveQuotesAndProofs(quote string, proofs []cashu.Proof) {
	if quote != "" {
		m.ActiveQuotes.RemoveQuote(quote)
	}

	if len(proofs) == 0 {
		m.ActiveProofs.RemoveProofs(proofs)

	}
}

// errors types for validation
var (
	ErrInvalidProof        = errors.New("Invalid proof")
	ErrQuoteNotPaid        = errors.New("Quote not paid")
	ErrMessageAmountToBig  = errors.New("Message amount is to big")
	ErrInvalidBlindMessage = errors.New("Invalid blind message")
)

var (
	NETWORK_ENV                = "NETWORK"
	MINT_LIGHTNING_BACKEND_ENV = "MINT_LIGHTNING_BACKEND"
)

func (m *Mint) CheckProofsAreSameUnit(proofs []cashu.Proof) (cashu.Unit, error) {

	units := make(map[string]bool)

	for _, proof := range proofs {

		keyset, err := m.GetKeysetById(proof.Id)
		if err != nil {
			return cashu.Sat, fmt.Errorf("GetKeysetById: %w", err)
		}

		if len(keyset) == 0 {
			return cashu.Sat, cashu.ErrKeysetForProofNotFound
		}

		units[keyset[0].Unit] = true
		if len(units) > 1 {
			return cashu.Sat, fmt.Errorf("Proofs are not the same unit")
		}
	}

	if len(units) == 0 {
		return cashu.Sat, fmt.Errorf("No units found")
	}

	var returnedUnit cashu.Unit
	for unit := range units {
		finalUnit, err := cashu.UnitFromString(unit)
		if err != nil {
			return cashu.Sat, fmt.Errorf("UnitFromString: %w", err)
		}

		returnedUnit = finalUnit
	}

	return returnedUnit, nil

}
func (m *Mint) VerifyListOfProofs(proofs []cashu.Proof, blindMessages []cashu.BlindedMessage, unit cashu.Unit) error {
	checkOutputs := false

	pubkeysFromProofs := make(map[*btcec.PublicKey]bool)

	for _, proof := range proofs {
		err := m.ValidateProof(proof, unit, &checkOutputs, &pubkeysFromProofs)
		if err != nil {
			return fmt.Errorf("ValidateProof: %w", err)
		}
	}

	// if any sig allis present all outputs also need to be check with the pubkeys from the proofs
	if checkOutputs {
		for _, blindMessage := range blindMessages {

			err := blindMessage.VerifyBlindMessageSignature(pubkeysFromProofs)
			if err != nil {
				return fmt.Errorf("blindMessage.VerifyBlindMessageSignature: %w", err)
			}

		}
	}

	return nil
}

func (m *Mint) ValidateProof(proof cashu.Proof, unit cashu.Unit, checkOutputs *bool, pubkeysFromProofs *map[*btcec.PublicKey]bool) error {
	var keysetToUse cashu.Keyset
	for _, keyset := range m.Keysets[unit.String()] {
		if keyset.Amount == proof.Amount && keyset.Id == proof.Id {
			keysetToUse = keyset
			break
		}
	}

	// check if keysetToUse is not assigned
	if keysetToUse.Id == "" {
		return cashu.ErrKeysetForProofNotFound
	}

	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, witness, err := proof.IsProofSpendConditioned(checkOutputs)

	if err != nil {
		return fmt.Errorf("proof.IsProofSpendConditioned(): %w", err)
	}

	if isProofLocked {
		ok, err := proof.VerifyWitness(spendCondition, witness, pubkeysFromProofs)

		if err != nil {
			return fmt.Errorf("proof.VerifyWitnessSig(): %w", err)
		}

		if !ok {
			return ErrInvalidProof
		}

	}

	parsedBlinding, err := hex.DecodeString(proof.C)

	if err != nil {
		return fmt.Errorf("hex.DecodeString: %w", err)
	}

	pubkey, err := secp256k1.ParsePubKey(parsedBlinding)
	if err != nil {
		return fmt.Errorf("secp256k1.ParsePubKey: %+v", err)
	}

	verified := crypto.Verify(proof.Secret, keysetToUse.PrivKey, pubkey)

	if !verified {
		return ErrInvalidProof
	}

	return nil
}

func (m *Mint) SignBlindedMessages(outputs []cashu.BlindedMessage, unit string) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	var blindedSignatures []cashu.BlindSignature
	var recoverSigDB []cashu.RecoverSigDB

	for _, output := range outputs {

		correctKeyset := m.ActiveKeysets[unit][output.Amount]

		blindSignature, err := output.GenerateBlindSignature(correctKeyset.PrivKey)

		recoverySig := cashu.RecoverSigDB{
			Amount:    output.Amount,
			Id:        output.Id,
			C_:        blindSignature.C_,
			B_:        output.B_,
			CreatedAt: time.Now().Unix(),
			Witness:   output.Witness,
		}

		if err != nil {
			err = fmt.Errorf("GenerateBlindSignature: %w %w", ErrInvalidBlindMessage, err)
			return nil, nil, err
		}

		blindedSignatures = append(blindedSignatures, blindSignature)
		recoverSigDB = append(recoverSigDB, recoverySig)

	}
	return blindedSignatures, recoverSigDB, nil
}

func (m *Mint) GetKeysetById(id string) ([]cashu.Keyset, error) {

	allKeys := m.GetAllKeysets()

	var keyset []cashu.Keyset

	for _, key := range allKeys {

		if key.Id == id {
			keyset = append(keyset, key)
		}
	}

	return keyset, nil
}

func (m *Mint) GetAllKeysets() []cashu.Keyset {
	var allKeys []cashu.Keyset

	for _, keyset := range m.Keysets {
		allKeys = append(allKeys, keyset...)
	}

	return allKeys
}

func (m *Mint) OrderActiveKeysByUnit() cashu.KeysResponse {
	// convert map to slice
	var keys []cashu.Keyset
	for _, keyset := range m.ActiveKeysets {
		for _, key := range keyset {
			keys = append(keys, key)
		}
	}

	orderedKeys := cashu.OrderKeysetByUnit(keys)

	return orderedKeys
}

func CheckChainParams(network string) (chaincfg.Params, error) {
	switch network {
	case "testnet":
		return chaincfg.TestNet3Params, nil
	case "mainnet":
		return chaincfg.MainNetParams, nil
	case "regtest":
		return chaincfg.RegressionNetParams, nil
	case "signet":
		return chaincfg.SigNetParams, nil
	default:
		return chaincfg.MainNetParams, fmt.Errorf("Invalid network: %s", network)
	}

}

func SetUpMint(ctx context.Context, mint_privkey string, seeds []cashu.Seed, config Config) (*Mint, error) {
	activeProofs := ActiveProofs{
		Proofs: make(map[cashu.Proof]bool),
	}
	activeQuotes := ActiveQuote{
		Quote: make(map[string]bool),
	}
	mint := Mint{
		ActiveKeysets: make(map[string]cashu.KeysetMap),
		Keysets:       make(map[string][]cashu.Keyset),
		Config:        config,
		ActiveProofs:  &activeProofs,
		ActiveQuotes:  &activeQuotes,
	}

	chainparam, err := CheckChainParams(config.NETWORK)
	if err != nil {
		return &mint, fmt.Errorf("CheckChainParams(config.NETWORK) %w", err)
	}

	switch config.MINT_LIGHTNING_BACKEND {

	case FAKE_WALLET:
		fake_wallet := lightning.FakeWallet{
			Network: chainparam,
		}

		mint.LightningBackend = fake_wallet

	case LNDGRPC:

		lndWallet := lightning.LndGrpcWallet{
			Network: chainparam,
		}

		err := lndWallet.SetupGrpc(config.LND_GRPC_HOST, config.LND_MACAROON, config.LND_TLS_CERT)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = lndWallet

	default:
		log.Fatalf("Unknown lightning backend: %s", config.MINT_LIGHTNING_BACKEND)
	}

	mint.PendingProofs = make([]cashu.Proof, 0)

	// uses seed to generate the keysets
	for _, seed := range seeds {

		// decrypt seed

		keysets, err := seed.DeriveKeyset(mint_privkey)
		if err != nil {
			return &mint, fmt.Errorf("seed.DeriveKeyset(mint_privkey): %w", err)
		}

		if seed.Active {
			mint.ActiveKeysets[seed.Unit] = make(cashu.KeysetMap)
			for _, keyset := range keysets {
				mint.ActiveKeysets[seed.Unit][keyset.Amount] = keyset
			}

		}

		mint.Keysets[seed.Unit] = append(mint.Keysets[seed.Unit], keysets...)
	}

	// parse mint private key and get hex value pubkey

	parsedBytes, err := hex.DecodeString(mint_privkey)

	if err != nil {
		return &mint, fmt.Errorf("Could not setup mints pubkey %w", err)
	}

	pubkeyhex := hex.EncodeToString(secp256k1.PrivKeyFromBytes(parsedBytes).PubKey().SerializeCompressed())

	mint.MintPubkey = pubkeyhex

	return &mint, nil
}

type AddToDBFunc func(*pgxpool.Pool, bool, cashu.ACTION_STATE, string) error

func (m *Mint) VerifyLightingPaymentHappened(pool *pgxpool.Pool, paid bool, quote string, dbCall AddToDBFunc) (cashu.ACTION_STATE, string, error) {
	state, preimage, err := m.LightningBackend.CheckPayed(quote)
	if err != nil {
		return cashu.UNPAID, "", fmt.Errorf("mint.LightningComs.CheckIfInvoicePayed: %w", err)
	}

	switch {
	case state == cashu.PAID:
		err := dbCall(pool, true, cashu.PAID, quote)
		if err != nil {
			return cashu.PAID, preimage, fmt.Errorf("dbCall: %w", err)
		}
		return cashu.PAID, preimage, nil

	case state == cashu.UNPAID:
		err := dbCall(pool, true, cashu.UNPAID, quote)
		if err != nil {
			return cashu.UNPAID, preimage, fmt.Errorf("dbCall: %w", err)
		}
		return cashu.UNPAID, preimage, nil

	}

	return cashu.UNPAID, "", nil
}
