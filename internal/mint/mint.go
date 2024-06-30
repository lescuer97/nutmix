package mint

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/tyler-smith/go-bip32"
	"log"
	"time"
)

type KeysetMap map[uint64]cashu.Keyset

type Mint struct {
	ActiveKeysets map[string]KeysetMap
	Keysets       map[string][]cashu.Keyset
	LightningComs comms.LightingComms
	Network       chaincfg.Params
	PendingProofs []cashu.Proof
}

// errors types for validation

var (
	ErrKeysetNotFound         = errors.New("Keyset not found")
	ErrKeysetForProofNotFound = errors.New("Keyset for proof not found")
	ErrInvalidProof           = errors.New("Invalid proof")
	ErrQuoteNotPaid           = errors.New("Quote not paid")
	ErrMessageAmountToBig     = errors.New("Message amount is to big")
	ErrInvalidBlindMessage    = errors.New("Invalid blind message")
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
			return cashu.Sat, ErrKeysetForProofNotFound
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
		return ErrKeysetForProofNotFound
	}

	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, witness, err := proof.IsProofSpendConditioned(checkOutputs)

	if err != nil {
		log.Printf("proof.IsProofSpendConditioned(): %+v", err)
		return fmt.Errorf("proof.IsProofSpendConditioned(): %w", err)
	}

	if isProofLocked {
		ok, err := proof.VerifyWitnessSig(spendCondition, witness, pubkeysFromProofs)

		if err != nil {
			return fmt.Errorf("proof.VerifyWitnessSig(): %w", err)
		}

		if !ok {
			return ErrInvalidProof
		}

	}

	parsedBlinding, err := hex.DecodeString(proof.C)

	if err != nil {
		log.Printf("hex.DecodeString: %+v", err)
		return err
	}

	pubkey, err := secp256k1.ParsePubKey(parsedBlinding)
	if err != nil {
		log.Printf("secp256k1.ParsePubKey: %+v", err)
		return err
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

func SetUpMint(ctx context.Context, mint_privkey string, seeds []cashu.Seed) (Mint, error) {
	mint := Mint{
		ActiveKeysets: make(map[string]KeysetMap),
		Keysets:       make(map[string][]cashu.Keyset),
	}

	network := ctx.Value(NETWORK_ENV)
	switch network {
	case "testnet":
		mint.Network = chaincfg.TestNet3Params
	case "mainnet":
		mint.Network = chaincfg.MainNetParams
	case "regtest":
		mint.Network = chaincfg.RegressionNetParams
	case "signet":
		mint.Network = chaincfg.SigNetParams
	default:
		return mint, fmt.Errorf("Invalid network: %s", network)
	}

	lightningBackendType := ctx.Value(MINT_LIGHTNING_BACKEND_ENV)
	switch lightningBackendType {

	case comms.FAKE_WALLET:

	case comms.LND_WALLET:
		lightningComs, err := comms.SetupLightingComms(ctx)

		if err != nil {
			return mint, err
		}
		mint.LightningComs = *lightningComs
	default:
		log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
	}

	mint.PendingProofs = make([]cashu.Proof, 0)

	// uses seed to generate the keysets
	for _, seed := range seeds {

		// decrypt seed

		// decrypt seed first
		err := seed.DecryptSeed(mint_privkey)

		if err != nil {
			log.Println(fmt.Errorf("seed.DecryptSeed: %w", err))
		}
		masterKey, err := bip32.NewMasterKey(seed.Seed)
		if err != nil {
			log.Println(fmt.Errorf("NewMasterKey: %w", err))
			return mint, err
		}

		unit, err := cashu.UnitFromString(seed.Unit)
		if err != nil {
			log.Println(fmt.Errorf("cashu.UnitFromString: %w", err))
			return mint, err
		}

		keysets, err := cashu.GenerateKeysets(masterKey, cashu.GetAmountsForKeysets(), seed.Id, unit)

		if err != nil {
			return mint, fmt.Errorf("GenerateKeysets: %w", err)
		}

		if seed.Active {
			mint.ActiveKeysets[seed.Unit] = make(KeysetMap)
			for _, keyset := range keysets {
				mint.ActiveKeysets[seed.Unit][keyset.Amount] = keyset
			}

		}

		mint.Keysets[seed.Unit] = append(mint.Keysets[seed.Unit], keysets...)
	}

	return mint, nil
}

type AddToDBFunc func(*pgxpool.Pool, bool, string) error

func (m *Mint) VerifyLightingPaymentHappened(ctx context.Context, pool *pgxpool.Pool, paid bool, quote string, dbCall AddToDBFunc) (bool, error) {
	lightningBackendType := ctx.Value(MINT_LIGHTNING_BACKEND_ENV)
	switch lightningBackendType {

	case comms.FAKE_WALLET:
		err := dbCall(pool, true, quote)
		if err != nil {
			return false, fmt.Errorf("dbCall: %w", err)
		}

		return true, nil

	case comms.LND_WALLET:
		invoiceDB, err := m.LightningComs.CheckIfInvoicePayed(quote)
		if err != nil {

			return false, fmt.Errorf("mint.LightningComs.CheckIfInvoicePayed: %w", err)
		}
		// if invoiceDB.
		if invoiceDB.State == lnrpc.Invoice_SETTLED {
			err := dbCall(pool, true, quote)
			if err != nil {
				return false, fmt.Errorf("dbCall: %w", err)
			}
			return true, nil

		} else {

			return false, nil
		}

	}
	return false, nil
}
