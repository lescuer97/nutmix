package localsigner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	gonutsC "github.com/elnosh/gonuts/cashu"
	"github.com/elnosh/gonuts/cashu/nuts/nut10"
	"github.com/elnosh/gonuts/cashu/nuts/nut11"
	"github.com/elnosh/gonuts/cashu/nuts/nut14"
	"github.com/elnosh/gonuts/crypto"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/tyler-smith/go-bip32"
)

type LocalSigner struct {
	// activeKeysets map[unit][]cashu.KeysetMap
	activeKeysets map[string]cashu.KeysetMap
	// keysets map[unit][]cashu.Keyset
	keysets map[string][]cashu.Keyset
	db      database.MintDB
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
	var keys []cashu.Keyset
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

	val, exists := l.keysets[unit.String()]
	if exists {
		return val, nil

	}
	return val, signer.ErrNoKeysetFound
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
	seeds, err := l.db.GetSeedsByUnit(cashu.Sat)
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
func (l *LocalSigner) GetPubkey() ([]byte, error) {
	key, err := l.getSignerPrivateKey()
	if err != nil {
		return []byte{}, fmt.Errorf("ParsePubKey: %w", err)
	}
	return key.PublicKey().PublicKey().Key, nil
}

func (l *LocalSigner) SignBlindMessages(messages gonutsC.BlindedMessages, unit cashu.Unit) (gonutsC.BlindedSignatures, error) {
	var blindedSignatures gonutsC.BlindedSignatures

	for _, message := range messages {
		correctKeyset := l.activeKeysets[unit.String()][message.Amount]

		if correctKeyset.PrivKey == nil || correctKeyset.Id != message.Id {
			return nil, cashu.UsingInactiveKeyset
		}

		decodedB_, err := hex.DecodeString(message.B_)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString(message.B_) %w", err)
		}

		B_, err := secp256k1.ParsePubKey(decodedB_)
		if err != nil {
			return nil, fmt.Errorf("secp256k1.ParsePubKey(decodedB_)  %w", err)
		}

		C_ := crypto.SignBlindedMessage(B_, correctKeyset.PrivKey)
		// blindSignature, err :=
		dleq_e, dleq_s := crypto.GenerateDLEQ(correctKeyset.PrivKey, B_, C_)

		dleq := gonutsC.DLEQProof{
			E: hex.EncodeToString(dleq_e.Serialize()),
			S: hex.EncodeToString(dleq_s.Serialize()),
		}

		blindSig := gonutsC.BlindedSignature{
			Amount: message.Amount,
			C_:     hex.EncodeToString(C_.SerializeCompressed()),
			Id:     message.Id,
			DLEQ:   &dleq,
		}
		blindedSignatures = append(blindedSignatures, blindSig)

	}
	return blindedSignatures, nil
}

func (l *LocalSigner) VerifyProofs(proofs gonutsC.Proofs, blindMessages gonutsC.BlindedMessages, unit cashu.Unit) error {
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

func (l *LocalSigner) validateProof(proof gonutsC.Proof, unit cashu.Unit, checkOutputs *bool, pubkeysFromProofs *map[*btcec.PublicKey]bool) error {
	var keysetToUse cashu.Keyset
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

	secret, err := nut10.DeserializeSecret(proof.Secret)
	if err != nil {
		return fmt.Errorf("nut10.DeserializeSecret(proof.Secret): %w", err)
	}

	if secret.Kind != nut10.AnyoneCanSpend {

		switch secret.Kind {
		case nut10.P2PK:
			var witness nut11.P2PKWitness
			err := json.Unmarshal([]byte(proof.Witness), &witness)
			if err != nil {
				return cashu.ErrInvalidProof
			}

			tags, err := nut11.ParseP2PKTags(secret.Data.Tags)
			if err != nil {
				return cashu.ErrInvalidProof
			}

			hashMessage := sha256.Sum256([]byte(proof.Secret))

			valid := nut11.HasValidSignatures(hashMessage[:], witness.Signatures, tags.NSigs, tags.Pubkeys)

			if !valid {
				return cashu.ErrInvalidProof
			}
		case nut10.HTLC:
			var witness nut14.HTLCWitness
			err := json.Unmarshal([]byte(proof.Witness), &witness)
			if err != nil {
				return fmt.Errorf("json.Unmarshal([]byte(proof.Witness), &witness): %w", err)
			}

			tags, err := nut11.ParseP2PKTags(secret.Data.Tags)
			if err != nil {
				return fmt.Errorf("nut11.ParseP2PKTags(secret.Data.Tags): %w", err)
			}

			hashMessage := sha256.Sum256([]byte(proof.Secret))

			valid := nut11.HasValidSignatures(hashMessage[:], witness.Signatures, tags.NSigs, tags.Pubkeys)

			if !valid {
				return cashu.ErrInvalidProof
			}

		}

	}

	// proof.Witness
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
