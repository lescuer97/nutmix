package localsigner

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/elnosh/gonuts/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/tyler-smith/go-bip32"
)

type LocalSigner struct {
	activeKeysets map[string]cashu.MintKeysMap
	keysets       map[string]cashu.MintKeysMap
	db            database.MintDB
}

func SetupLocalSigner(db database.MintDB) (LocalSigner, error) {
	localsigner := LocalSigner{
		db: db,
	}

	privateKey, err := localsigner.getSignerPrivateKey()
	if err != nil {
		return localsigner, fmt.Errorf("signer.getSignerPrivateKey(). %w", err)
	}
	masterKey, err := bip32.NewMasterKey(privateKey.Serialize())
	if err != nil {
		return localsigner, fmt.Errorf(" bip32.NewMasterKey(privateKey.Serialize()). %w", err)
	}
	seeds, err := localsigner.db.GetAllSeeds()
	if err != nil {
		return localsigner, fmt.Errorf("signer.db.GetAllSeeds(). %w", err)
	}
	if len(seeds) == 0 {
		newSeed, err := localsigner.createNewSeed(masterKey, cashu.Sat, 1, 0)

		if err != nil {
			return localsigner, fmt.Errorf("signer.createNewSeed(masterKey, 1, 0). %w", err)
		}

		err = db.SaveNewSeeds([]cashu.Seed{newSeed})
		if err != nil {
			return localsigner, fmt.Errorf("db.SaveNewSeeds([]cashu.Seed{newSeed}). %w", err)
		}
		seeds = append(seeds, newSeed)

	}
	keysets, activeKeysets, err := signer.GetKeysetsFromSeeds(seeds, masterKey)
	if err != nil {
		return localsigner, fmt.Errorf(`signer.GetKeysetsFromSeeds(seeds, masterKey). %w`, err)
	}

	localsigner.keysets = keysets
	localsigner.activeKeysets = activeKeysets

	masterKey = nil
	return localsigner, nil

}

// gets all active keys
func (l *LocalSigner) GetActiveKeys() (signer.GetKeysResponse, error) {
	// convert map to slice
	var keys []cashu.MintKey
	for _, keyset := range l.activeKeysets {
		for _, key := range keyset {
			if key.Unit != cashu.AUTH.String() {
				keys = append(keys, key)
			}
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Amount < keys[j].Amount
	})

	return signer.OrderKeysetByUnit(keys), nil
}

func (l *LocalSigner) GetKeysById(id string) (signer.GetKeysResponse, error) {

	val, exists := l.keysets[id]
	if exists {
		var keys []cashu.MintKey
		for _, key := range val {
			if key.Unit != cashu.AUTH.String() {
				keys = append(keys, key)
			}
		}

		return signer.OrderKeysetByUnit(keys), nil

	}
	return signer.GetKeysResponse{}, signer.ErrNoKeysetFound
}

// gets all keys from the signer
func (l *LocalSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	seeds, err := l.db.GetAllSeeds()
	if err != nil {
		return response, fmt.Errorf(" l.db.GetAllSeeds(). %w", err)
	}
	for _, seed := range seeds {
		if seed.Unit != cashu.AUTH.String() {
			response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active, InputFeePpk: seed.InputFeePpk, Version: uint64(seed.Version)})
		}
	}
	return response, nil
}

func (l *LocalSigner) getSignerPrivateKey() (*secp256k1.PrivateKey, error) {
	mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
	if mint_privkey == "" {
		return nil, fmt.Errorf(`os.Getenv("MINT_PRIVATE_KEY").`)
	}

	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
		return nil, fmt.Errorf(`hex.DecodeString(mint_privkey). %w`, err)
	}
	mintKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	return mintKey, nil
}

func (l *LocalSigner) createNewSeed(mintPrivateKey *bip32.Key, unit cashu.Unit, version int, fee uint) (cashu.Seed, error) {
	// rotate one level up
	newSeed := cashu.Seed{
		CreatedAt:   time.Now().Unix(),
		Active:      true,
		Version:     version,
		Unit:        unit.String(),
		InputFeePpk: fee,
	}

	keysets, err := signer.DeriveKeyset(mintPrivateKey, newSeed)
	if err != nil {
		return newSeed, fmt.Errorf("DeriveKeyset(mintPrivateKey, newSeed) %w", err)
	}
	newSeedId, err := cashu.DeriveKeysetId(keysets)
	if err != nil {
		return newSeed, fmt.Errorf("cashu.DeriveKeysetId(keyset) %w", err)
	}

	newSeed.Id = newSeedId
	return newSeed, nil

}

func (l *LocalSigner) RotateKeyset(unit cashu.Unit, fee uint, expiry_limit uint) error {
	ctx := context.Background()
	tx, err := l.db.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("l.db.GetTx(ctx). %w", err)
	}
	defer l.db.Rollback(ctx, tx)

	// get current highest seed version
	var highestSeed cashu.Seed = cashu.Seed{Version: 0}
	seeds, err := l.db.GetSeedsByUnit(tx, unit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("database.GetSeedsByUnit(tx, unit). %w", err)

		}
	}

	// get current highest seed version
	for i, seed := range seeds {
		if highestSeed.Version < seed.Version {
			highestSeed = seed
		}

		seeds[i].Active = false
	}

	mintPrivateKey, err := l.getSignerPrivateKey()
	if err != nil {
		return fmt.Errorf(`l.getSignerPrivateKey() %w`, err)
	}

	signerMasterKey, err := bip32.NewMasterKey(mintPrivateKey.Serialize())
	if err != nil {
		return fmt.Errorf(" bip32.NewMasterKey(mintPrivateKey.Serialize()). %w", err)
	}

	// Create New seed with one higher version
	newSeed, err := l.createNewSeed(signerMasterKey, unit, highestSeed.Version+1, fee)

	if err != nil {
		return fmt.Errorf(`l.createNewSeed(signerMasterKey, unit, highestSeed.Version+1, fee) %w`, err)
	}

	// add new key to db
	err = l.db.SaveNewSeed(tx, newSeed)
	if err != nil {
		return fmt.Errorf(`l.db.SaveNewSeed(tx, newSeed). %w`, err)
	}

	// only need to update if there are any previous seeds
	if len(seeds) > 0 {
		err = l.db.UpdateSeedsActiveStatus(tx, seeds)
		if err != nil {
			return fmt.Errorf(`l.db.UpdateSeedsActiveStatus(tx, seeds). %w`, err)
		}
	}

	err = l.db.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf(`l.db.Commit(ctx, tx). %w`, err)
	}
	seeds, err = l.db.GetAllSeeds()
	if err != nil {
		return fmt.Errorf("signer.db.GetAllSeeds(). %w", err)
	}

	keysets, activeKeysets, err := signer.GetKeysetsFromSeeds(seeds, signerMasterKey)
	if err != nil {
		return fmt.Errorf(`m.DeriveKeysetFromSeeds(seeds, parsedPrivateKey). %w`, err)
	}

	l.keysets = keysets
	l.activeKeysets = activeKeysets

	signerMasterKey = nil
	return nil
}

func (l *LocalSigner) signBlindMessage(k *secp256k1.PrivateKey, message cashu.BlindedMessage) (cashu.BlindSignature, error) {
	var blindSignature cashu.BlindSignature

	decodedBlindFactor, err := hex.DecodeString(message.B_)

	if err != nil {
		return blindSignature, fmt.Errorf("DecodeString: %w", err)
	}

	B_, err := secp256k1.ParsePubKey(decodedBlindFactor)

	if err != nil {
		return blindSignature, fmt.Errorf("ParsePubKey: %w", err)
	}

	C_ := crypto.SignBlindedMessage(B_, k)

	blindSig := cashu.BlindSignature{
		Amount: message.Amount,
		Id:     message.Id,
		C_:     hex.EncodeToString(C_.SerializeCompressed()),
	}

	err = blindSig.GenerateDLEQ(B_, k)

	if err != nil {
		return blindSig, fmt.Errorf("blindSig.GenerateDLEQ: %w", err)
	}

	return blindSig, nil
}

func (l *LocalSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	var blindedSignatures []cashu.BlindSignature
	var recoverSigDB []cashu.RecoverSigDB

	for _, output := range messages {
		correctKeyset := l.activeKeysets[output.Id][output.Amount]

		if correctKeyset.PrivKey == nil || !correctKeyset.Active {
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
			err = errors.Join(cashu.ErrInvalidBlindMessage, err)
			return nil, nil, err
		}

		blindedSignatures = append(blindedSignatures, blindSignature)
		recoverSigDB = append(recoverSigDB, recoverySig)

	}
	return blindedSignatures, recoverSigDB, nil

}

func (l *LocalSigner) VerifyProofs(proofs []cashu.Proof) error {

	for _, proof := range proofs {
		err := l.validateProof(proof)
		if err != nil {
			return fmt.Errorf("l.validateProof(proof, unit, &checkOutputs, &pubkeysFromProofs): %w", err)
		}
	}

	return nil
}

func (l *LocalSigner) validateProof(proof cashu.Proof) error {
	var keysetToUse cashu.MintKey

	keysets, exists := l.keysets[proof.Id]
	if !exists {
		return cashu.ErrKeysetForProofNotFound
	}

	for _, keyset := range keysets {
		if keyset.Amount == proof.Amount && keyset.Id == proof.Id {
			keysetToUse = keyset
			break
		}
	}

	// check if keysetToUse is not assigned
	if keysetToUse.Id == "" {
		return cashu.ErrKeysetForProofNotFound
	}
	parsedBlinding, err := hex.DecodeString(proof.C)
	if err != nil {
		return fmt.Errorf("hex.DecodeString: %w", errors.Join(err, cashu.ErrInvalidProof))
	}
	pubkey, err := secp256k1.ParsePubKey(parsedBlinding)
	if err != nil {
		return fmt.Errorf("secp256k1.ParsePubKey: %w", errors.Join(err, cashu.ErrInvalidProof))
	}
	verified := crypto.Verify(proof.Secret, keysetToUse.PrivKey, pubkey)
	if !verified {
		return cashu.ErrInvalidProof
	}

	return nil

}
func (l *LocalSigner) GetSignerPubkey() (string, error) {

	mintPrivateKey, err := l.getSignerPrivateKey()
	if err != nil {
		return "", fmt.Errorf(`l.getSignerPrivateKey() %w`, err)
	}

	return hex.EncodeToString(mintPrivateKey.PubKey().SerializeCompressed()), nil
}

// gets all active keys
func (l *LocalSigner) GetAuthActiveKeys() (signer.GetKeysResponse, error) {
	// convert map to slice
	var keys []cashu.MintKey
	for _, keyset := range l.activeKeysets {
		for _, key := range keyset {
			if key.Unit == cashu.AUTH.String() {
				keys = append(keys, key)
			}
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Amount < keys[j].Amount
	})

	return signer.OrderKeysetByUnit(keys), nil
}

func (l *LocalSigner) GetAuthKeysById(id string) (signer.GetKeysResponse, error) {

	val, exists := l.keysets[id]
	if exists {
		var keys []cashu.MintKey
		for _, key := range val {
			if key.Unit == cashu.AUTH.String() {
				keys = append(keys, key)
			}
		}

		return signer.OrderKeysetByUnit(keys), nil

	}
	return signer.GetKeysResponse{}, signer.ErrNoKeysetFound
}

// gets all keys from the signer
func (l *LocalSigner) GetAuthKeys() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	seeds, err := l.db.GetAllSeeds()
	if err != nil {
		return response, fmt.Errorf(" l.db.GetAllSeeds(). %w", err)
	}
	for _, seed := range seeds {
		if seed.Unit == cashu.AUTH.String() {
			response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active, InputFeePpk: seed.InputFeePpk, Version: uint64(seed.Version)})
		}
	}
	return response, nil
}
