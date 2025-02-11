package mint

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
)

type Mint struct {
	ActiveKeysets    map[string]cashu.KeysetMap
	Keysets          map[string][]cashu.Keyset
	LightningBackend lightning.LightningBackend
	ActiveQuotes     *ActiveQuote
	Config           utils.Config
	MintPubkey       string
	MintDB           database.MintDB
}

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
			return cashu.AlreadyActiveProof
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
		return cashu.AlreadyActiveQuote
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

	return nil
}

func (m *Mint) RemoveQuotesAndProofs(quote string, proofs []cashu.Proof) {
	if quote != "" {
		m.ActiveQuotes.RemoveQuote(quote)
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

func (m *Mint) CheckMeltQuoteState(quoteId string) (cashu.MeltRequestDB, error) {
	ctx := context.Background()
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer tx.Rollback(ctx)
	quote, err := m.MintDB.GetMeltRequestById(tx, quoteId)

	if err != nil {
		return quote, fmt.Errorf("m.MintDB.GetMeltRequestById(quoteId). %w", err)
	}

	if quote.State == cashu.PENDING {

		status, preimage, fee, err := m.LightningBackend.CheckPayed(quote.Quote)
		if err != nil {
			return quote, fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote). %w", err)
		}

		if status == lightning.SETTLED {
			quote.State = cashu.PAID
			quote.PaidFee = fee
			quote.PaymentPreimage = preimage

			pending_proofs, err := m.MintDB.GetProofsFromQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
			}
			changeMessages, err := m.MintDB.GetMeltChangeByQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
			}

			fee, err := cashu.Fees(pending_proofs, m.Keysets[quote.Unit])
			if err != nil {
				return quote, fmt.Errorf("cashu.Fees(pending_proofs, m.Keysets[quote.Unit]). %w", err)
			}

			totalExpent := quote.Amount + quote.PaidFee + uint64(fee)

			overpaidFees := pending_proofs.Amount() - totalExpent

			if len(changeMessages) > 0 && overpaidFees > 0 {

				var blindMessages []cashu.BlindedMessage
				for _, v := range changeMessages {
					blindMessages = append(blindMessages, cashu.BlindedMessage{Id: v.Id, B_: v.Id})

				}
				sigs, err := m.GetChangeOutput(blindMessages, overpaidFees, quote.Unit)
				if err != nil {
					return quote, fmt.Errorf("m.GetChangeOutput(changeMessages, quote.Unit ). %w", err)
				}

				err = m.MintDB.SaveRestoreSigs(tx, sigs)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.SaveRestoreSigs(sigs) %w", err)
				}

				err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(quote.Quote) %w", err)
				}

			}

			err = m.MintDB.SetProofsState(tx, pending_proofs, cashu.PROOF_SPENT)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.SetProofsState(pending_proofs, cashu.PROOF_SPENT) %w", err)
			}

			err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

		}
		if status == lightning.FAILED {
			quote.State = cashu.UNPAID
			pending_proofs, err := m.MintDB.GetProofsFromQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
			}

			err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

			err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(quote.Quote) %w", err)
			}
			err = m.MintDB.DeleteProofs(tx, pending_proofs)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.DeleteProofsByQuote(quote.Quote). %w", err)
			}

		}

	}

	err = tx.Commit(context.Background())
	if err != nil {
		return quote, fmt.Errorf("tx.Commit(context.Background()). %w", err)
	}
	return quote, nil
}

func (m *Mint) CheckPendingQuoteAndProofs(logger *slog.Logger) error {

	quotes, err := m.MintDB.GetMeltQuotesByState(cashu.PENDING)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetMeltQuotesByState(cashu.PENDING). %w", err)
	}

	for _, quote := range quotes {
		quote, err := m.CheckMeltQuoteState(quote.Quote)
		if err != nil {
			return fmt.Errorf("m.MintDB.GetMeltQuotesByState(cashu.PENDING). %w", err)
		}

		logger.Info(fmt.Sprintf("Melt quote %v state: %v", quote.Quote, quote.State))

	}

	return nil
}

func (m *Mint) SignBlindedMessages(outputs []cashu.BlindedMessage, unit string) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	var blindedSignatures []cashu.BlindSignature
	var recoverSigDB []cashu.RecoverSigDB

	for _, output := range outputs {

		correctKeyset := m.ActiveKeysets[unit][output.Amount]

		if correctKeyset.PrivKey == nil || correctKeyset.Id != output.Id {
			return nil, nil, cashu.UsingInactiveKeyset
		}

		blindSignature, err := output.GenerateBlindSignature(correctKeyset.PrivKey)

		recoverySig := cashu.RecoverSigDB{
			Amount:    output.Amount,
			Id:        output.Id,
			C_:        blindSignature.C_,
			B_:        output.B_,
			Dleq:      blindSignature.Dleq,
			CreatedAt: time.Now().Unix(),
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

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Amount < keys[j].Amount
	})

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

func SetUpMint(ctx context.Context, mint_privkey *secp256k1.PrivateKey, seeds []cashu.Seed, config utils.Config, db database.MintDB) (*Mint, error) {
	activeQuotes := ActiveQuote{
		Quote: make(map[string]bool),
	}
	mint := Mint{
		ActiveKeysets: make(map[string]cashu.KeysetMap),
		Keysets:       make(map[string][]cashu.Keyset),
		Config:        config,
		ActiveQuotes:  &activeQuotes,
		MintDB:        db,
	}

	chainparam, err := CheckChainParams(config.NETWORK)
	if err != nil {
		return &mint, fmt.Errorf("CheckChainParams(config.NETWORK) %w", err)
	}

	switch config.MINT_LIGHTNING_BACKEND {

	case utils.FAKE_WALLET:
		fake_wallet := lightning.FakeWallet{
			Network: chainparam,
		}

		mint.LightningBackend = fake_wallet

	case utils.LNDGRPC:
		lndWallet := lightning.LndGrpcWallet{
			Network: chainparam,
		}

		err := lndWallet.SetupGrpc(config.LND_GRPC_HOST, config.LND_MACAROON, config.LND_TLS_CERT)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = lndWallet
	case utils.LNBITS:
		lnbitsWallet := lightning.LnbitsWallet{
			Network:  chainparam,
			Endpoint: config.MINT_LNBITS_ENDPOINT,
			Key:      config.MINT_LNBITS_KEY,
		}
		mint.LightningBackend = lnbitsWallet
	case utils.CLNGRPC:
		clnWallet := lightning.CLNGRPCWallet{
			Network: chainparam,
		}

		err := clnWallet.SetupGrpc(config.CLN_GRPC_HOST, config.CLN_CA_CERT, config.CLN_CLIENT_CERT, config.CLN_CLIENT_KEY, config.CLN_MACAROON)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = clnWallet

	default:
		log.Fatalf("Unknown lightning backend: %s", config.MINT_LIGHTNING_BACKEND)
	}

	mintKey, err := MintPrivateKeyToBip32(mint_privkey)
	if err != nil {
		return &mint, fmt.Errorf("MintPrivateKeyToBip32(mint_privkey) %w", err)
	}

	allKeysets, activeKeyset, err := GetKeysetsFromSeeds(seeds, mintKey)
	if err != nil {
		return &mint, fmt.Errorf("DeriveKeysetFromSeeds(seeds, mint_privkey) %w", err)
	}

	mint.ActiveKeysets = activeKeyset
	mint.Keysets = allKeysets

	// parse mint private key and get hex value pubkey
	pubkeyhex := hex.EncodeToString(mint_privkey.PubKey().SerializeCompressed())

	mint.MintPubkey = pubkeyhex

	return &mint, nil
}

func GetKeysetsFromSeeds(seeds []cashu.Seed, mintKey *bip32.Key) (map[string][]cashu.Keyset, map[string]cashu.KeysetMap, error) {
	newKeysets := make(map[string][]cashu.Keyset)
	newActiveKeysets := make(map[string]cashu.KeysetMap)

	for _, seed := range seeds {

		keysets, err := DeriveKeyset(mintKey, seed)
		if err != nil {
			return newKeysets, newActiveKeysets, fmt.Errorf("DeriveKeyset(mintKey, seed) %w", err)
		}
		if seed.Active {
			newActiveKeysets[seed.Unit] = make(cashu.KeysetMap)
			for _, keyset := range keysets {
				newActiveKeysets[seed.Unit][keyset.Amount] = keyset
			}

		}

		newKeysets[seed.Unit] = append(newKeysets[seed.Unit], keysets...)
	}
	return newKeysets, newActiveKeysets, nil

}

func MintPrivateKeyToBip32(mintKey *secp256k1.PrivateKey) (*bip32.Key, error) {
	masterKey, err := bip32.NewMasterKey(mintKey.Serialize())
	if err != nil {

		return nil, fmt.Errorf(" bip32.NewMasterKey(mintKey.Serialize()). %w", err)
	}

	return masterKey, nil
}

func DeriveKeyset(mintKey *bip32.Key, seed cashu.Seed) ([]cashu.Keyset, error) {
	unit, err := cashu.UnitFromString(seed.Unit)
	if err != nil {

		return nil, fmt.Errorf("UnitFromString(seed.Unit) %w", err)
	}

	unitKey, err := mintKey.NewChildKey(uint32(unit.EnumIndex()))

	if err != nil {

		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(unit.EnumIndex())). %w", err)
	}

	versionKey, err := unitKey.NewChildKey(uint32(seed.Version))
	if err != nil {
		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(seed.Version)) %w", err)
	}

	keyset, err := cashu.GenerateKeysets(versionKey, cashu.GetAmountsForKeysets(), seed.Id, unit, seed.InputFeePpk, seed.Active)
	if err != nil {
		return nil, fmt.Errorf(`GenerateKeysets(versionKey,GetAmountsForKeysets(), "", unit, 0) %w`, err)
	}

	return keyset, nil
}

func CreateNewSeed(mintPrivateKey *bip32.Key, version int, fee int) (cashu.Seed, error) {
	// rotate one level up
	newSeed := cashu.Seed{
		CreatedAt:   time.Now().Unix(),
		Active:      true,
		Version:     version,
		Unit:        cashu.Sat.String(),
		InputFeePpk: fee,
	}

	keyset, err := DeriveKeyset(mintPrivateKey, newSeed)

	if err != nil {
		return newSeed, fmt.Errorf("DeriveKeyset(mintPrivateKey, newSeed) %w", err)
	}

	newSeedId, err := cashu.DeriveKeysetId(keyset)
	if err != nil {
		return newSeed, fmt.Errorf("cashu.DeriveKeysetId(keyset) %w", err)
	}

	newSeed.Id = newSeedId
	return newSeed, nil

}
