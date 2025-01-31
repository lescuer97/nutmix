package localsigner

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/elnosh/gonuts/crypto"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/tyler-smith/go-bip32"
)

type LocalSigner struct {
	activeKeysets map[string]cashu.MintKeysMap
	keysets       map[string][]cashu.MintKey
	db            database.MintDB
}

func SetupLocalSigner(db database.MintDB) (LocalSigner, error) {
	localsigner := LocalSigner{
		db: db,
	}

	masterKey, err := localsigner.getSignerPrivateKey()
	if err != nil {
		return localsigner, fmt.Errorf("signer.getSignerPrivateKey(). %w", err)
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
			keys = append(keys, key)
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

		return signer.OrderKeysetByUnit(val), nil

	}
	return signer.GetKeysResponse{}, signer.ErrNoKeysetFound
}
func (l *LocalSigner) GetKeysByUnit(unit cashu.Unit) ([]cashu.Keyset, error) {

	var keys []cashu.Keyset

	for _, mintKey := range l.keysets {

		if len(mintKey) > 0 {

			if mintKey[0].Unit == unit.String() {

				keysetResp := cashu.Keyset{
					Id:          mintKey[0].Id,
					Unit:        mintKey[0].Unit,
					InputFeePpk: mintKey[0].InputFeePpk,
				}

				for _, keyset := range mintKey {
					keysetResp.Keys[strconv.FormatUint(keyset.Amount, 10)] = hex.EncodeToString(keyset.PrivKey.PubKey().SerializeCompressed())
				}

				keys = append(keys, keysetResp)
			}

		}
	}
	return keys, nil
}

// gets all keys from the signer
func (l *LocalSigner) GetKeys() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	seeds, err := l.db.GetAllSeeds()
	if err != nil {
		return response, fmt.Errorf(" l.db.GetAllSeeds(). %w", err)
	}
	for _, seed := range seeds {
		response.Keysets = append(response.Keysets, signer.BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active, InputFeePpk: seed.InputFeePpk})
	}
	return response, nil
}

func (l *LocalSigner) getSignerPrivateKey() (*bip32.Key, error) {
	mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
	if mint_privkey == "" {
		return &bip32.Key{}, fmt.Errorf(`os.Getenv("MINT_PRIVATE_KEY").`)
	}

	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
		return &bip32.Key{}, fmt.Errorf(`hex.DecodeString(mint_privkey). %w`, err)
	}
	mintKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := bip32.NewMasterKey(mintKey.Serialize())
	if err != nil {
		return nil, fmt.Errorf(" bip32.NewMasterKey(mintKey.Serialize()). %w", err)
	}

	return masterKey, nil
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

	keyset, err := signer.DeriveKeyset(mintPrivateKey, newSeed)

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

func (l *LocalSigner) RotateKeyset(unit cashu.Unit, fee uint) error {
	seeds, err := l.db.GetSeedsByUnit(unit)
	if err != nil {
		return fmt.Errorf("database.GetSeedsByUnit(pool, cashu.Sat). %w", err)
	}
	// get current highest seed version
	var highestSeed cashu.Seed
	for i, seed := range seeds {
		if highestSeed.Version < seed.Version {
			highestSeed = seed
		}
		seeds[i].Active = false
	}

	signerMasterKey, err := l.getSignerPrivateKey()
	if err != nil {
		return fmt.Errorf(`l.getSignerPrivateKey() %w`, err)
	}

	// Create New seed with one higher version
	newSeed, err := l.createNewSeed(signerMasterKey, unit, highestSeed.Version+1, fee)

	if err != nil {
		return fmt.Errorf(`m.CreateNewSeed(masterKey,1,0 ) %w`, err)
	}

	// add new key to db
	err = l.db.SaveNewSeed(newSeed)
	if err != nil {
		return fmt.Errorf(`database.SaveNewSeed(pool, &generatedSeed). %w`, err)
	}
	err = l.db.UpdateSeedsActiveStatus(seeds)
	if err != nil {
		return fmt.Errorf(`database.UpdateActiveStatusSeeds(pool, seeds). %w`, err)
	}

	seeds = append(seeds, newSeed)

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
		correctKeyset := l.keysets[output.Id][output.Amount]

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
			err = fmt.Errorf("GenerateBlindSignature: %w %w", cashu.ErrInvalidBlindMessage, err)
			return nil, nil, err
		}

		blindedSignatures = append(blindedSignatures, blindSignature)
		recoverSigDB = append(recoverSigDB, recoverySig)

	}
	return blindedSignatures, recoverSigDB, nil

}

func (l *LocalSigner) VerifyProofs(proofs []cashu.Proof, blindMessages []cashu.BlindedMessage, unit cashu.Unit) error {
	checkOutputs := false

	pubkeysFromProofs := make(map[*btcec.PublicKey]bool)

	for _, proof := range proofs {
		err := l.validateProof(proof, unit, &checkOutputs, &pubkeysFromProofs)
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

func (l *LocalSigner) validateProof(proof cashu.Proof, unit cashu.Unit, checkOutputs *bool, pubkeysFromProofs *map[*btcec.PublicKey]bool) error {
	var keysetToUse cashu.MintKey
	for _, keyset := range l.keysets[unit.String()] {
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
			return cashu.ErrInvalidProof
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
		return cashu.ErrInvalidProof
	}

	return nil

}
