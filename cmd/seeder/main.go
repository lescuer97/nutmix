package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
)

const (
	DefaultMintPrivateKey = "0000000000000000000000000000000000000000000000000000000000000001"
)

func main() {
	log.Println("Starting database seeder...")

	// 1. Setup
	mintPrivateKeyHex := os.Getenv("MINT_PRIVATE_KEY")
	if mintPrivateKeyHex == "" {
		mintPrivateKeyHex = DefaultMintPrivateKey
	}

	decodedPrivKey, err := hex.DecodeString(mintPrivateKeyHex)
	if err != nil {
		log.Fatalf("Failed to decode mint private key: %v", err)
	}
	mintPrivKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := bip32.NewMasterKey(mintPrivKey.Serialize())
	if err != nil {
		log.Fatalf("Failed to create master key: %v", err)
	}

	db, err := postgresql.DatabaseSetup(context.Background(), "migrations")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 2. Main Loop - 24 Months
	now := time.Now()
	startTime := now.AddDate(0, -24, 0)

	availableProofs := make([]cashu.Proof, 0)
	var currentKeyset map[uint64]cashu.MintKey

	for i := 0; i < 24; i++ {
		currentMonthTime := startTime.AddDate(0, i, 0)
		log.Printf("Processing month: %s", currentMonthTime.Format("2006-01"))

		// --- Keyset Rotation ---
		fee := uint((i + 1) * 100)

		// Start Transaction for Keyset Rotation
		tx, err := db.GetTx(ctx)
		if err != nil {
			log.Fatalf("Failed to begin transaction: %v", err)
		}

		// Deactivate existing Sat seeds
		seeds, err := db.GetSeedsByUnit(tx, cashu.Sat)
		if err != nil {
			log.Fatalf("Failed to get seeds: %v", err)
		}

		highestVersion := 0
		for idx, s := range seeds {
			if s.Version > highestVersion {
				highestVersion = s.Version
			}
			seeds[idx].Active = false
		}

		if len(seeds) > 0 {
			if err := db.UpdateSeedsActiveStatus(tx, seeds); err != nil {
				log.Fatalf("Failed to update seeds status: %v", err)
			}
		}

		// Create New Seed
		newSeed := cashu.Seed{
			CreatedAt:   currentMonthTime.Unix(),
			Active:      true,
			Version:     highestVersion + 1,
			Unit:        cashu.Sat.String(),
			InputFeePpk: fee,
		}

		// Derive keys for the new seed
		keysets, err := signer.DeriveKeyset(masterKey, newSeed)
		if err != nil {
			log.Fatalf("Failed to derive keyset: %v", err)
		}

		// Calculate ID
		pubkeys := make([]*secp256k1.PublicKey, 0)
		for _, k := range keysets {
			pubkeys = append(pubkeys, k.GetPubKey())
		}
		keysetId, err := cashu.DeriveKeysetId(pubkeys)
		if err != nil {
			log.Fatalf("Failed to derive keyset ID: %v", err)
		}
		newSeed.Id = keysetId

		// Save new seed
		if err := db.SaveNewSeed(tx, newSeed); err != nil {
			log.Fatalf("Failed to save new seed: %v", err)
		}

		if err := db.Commit(ctx, tx); err != nil {
			log.Fatalf("Failed to commit keyset rotation: %v", err)
		}

		// Update current keyset map for signing
		currentKeyset = make(map[uint64]cashu.MintKey)
		for _, k := range keysets {
			// Ensure private key is set correctly from derivation
			k.Id = keysetId // Ensure ID is set
			currentKeyset[k.Amount] = k
		}

		// --- Request Generation ---
		numRequests := 50 // Max 50

		for j := 0; j < numRequests; j++ {
			// Randomly choose Mint or Melt
			isMint := coinFlip()

			if isMint {
				processMint(ctx, db, currentMonthTime, currentKeyset, keysetId, &availableProofs)
			} else {
				processMelt(ctx, db, currentMonthTime, &availableProofs)
			}
		}
	}

	log.Println("Database seeding completed successfully.")
}

func coinFlip() bool {
	n, _ := rand.Int(rand.Reader, big.NewInt(2))
	return n.Int64() == 0
}

func randomInt(max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}

func processMint(ctx context.Context, db postgresql.Postgresql, timestamp time.Time, keyset map[uint64]cashu.MintKey, keysetId string, availableProofs *[]cashu.Proof) {
	// Create Mint Request
	quoteId, _ := generateRandomHex(16)
	amount := uint64((randomInt(100) + 1) * 1000) // 1000 - 100000 sats

	stateChoice := randomInt(3) // 0: UNPAID, 1: PAID, 2: ISSUED
	var state cashu.ACTION_STATE
	var minted bool
	var paid bool

	switch stateChoice {
	case 0:
		state = cashu.UNPAID
		minted = false
		paid = false
	case 1:
		state = cashu.PAID // Assuming PAID means paid but not yet minted/issued
		minted = false
		paid = true
	case 2:
		state = cashu.ISSUED
		minted = true
		paid = true
	}

	req := cashu.MintRequestDB{
		Quote:       quoteId,
		Request:     "lnbcrt" + quoteId, // Fake bolt11
		RequestPaid: paid,
		Expiry:      timestamp.Add(time.Hour * 24).Unix(),
		Unit:        cashu.Sat.String(),
		Minted:      minted,
		State:       state,
		SeenAt:      timestamp.Unix(),
		Amount:      &amount,
		CheckingId:  quoteId,
		// Pubkey not strictly needed for this simulation
	}

	tx, err := db.GetTx(ctx)
	if err != nil {
		log.Printf("Mint: Failed to get tx: %v", err)
		return
	}
	defer db.Rollback(ctx, tx)

	if err := db.SaveMintRequest(tx, req); err != nil {
		log.Printf("Mint: Failed to save request: %v", err)
		return
	}

	if state == cashu.ISSUED {
		// Generate Blinded Messages
		blindedMessages, secrets, rs, err := createBlindedMessages(amount, keysetId)
		if err != nil {
			log.Printf("Mint: Failed to create blinded messages: %v", err)
			return
		}

		// Sign Messages
		var signatures []cashu.BlindSignature
		var recoverSigs []cashu.RecoverSigDB

		for i, msg := range blindedMessages {
			key, ok := keyset[msg.Amount]
			if !ok {
				log.Printf("Mint: Key not found for amount %d", msg.Amount)
				return
			}

			// Sign
			C_ := crypto.SignBlindedMessage(msg.B_.PublicKey, key.PrivKey)

			blindSig := cashu.BlindSignature{
				Amount: msg.Amount,
				Id:     keysetId,
				C_:     cashu.WrappedPublicKey{PublicKey: C_},
			}

			// Generate DLEQ
			if err := blindSig.GenerateDLEQ(msg.B_.PublicKey, key.PrivKey); err != nil {
				log.Printf("Mint: Failed to generate DLEQ: %v", err)
				return
			}

			signatures = append(signatures, blindSig)

			recoverSigs = append(recoverSigs, cashu.RecoverSigDB{
				Amount:    msg.Amount,
				Id:        keysetId,
				B_:        msg.B_,
				C_:        blindSig.C_,
				CreatedAt: timestamp.Unix(),
				Dleq:      blindSig.Dleq,
				MeltQuote: "", // Not relevant for mint
			})

			// Unblind to get proof
			C := crypto.UnblindSignature(C_, rs[i], key.PrivKey.PubKey())
			*availableProofs = append(*availableProofs, cashu.Proof{
				Amount: msg.Amount,
				Id:     keysetId,
				Secret: secrets[i],
				C:      cashu.WrappedPublicKey{PublicKey: C},
				// Y is needed for validation/DB constraint?
				// Usually Y = HashToCurve(Secret), let's calculate it if needed by DB schema
			})

			// Calculate Y for the proof (needed for DB SaveProof)
			// In cashu/nutmix, Y seems to be stored in DB proofs table.
			// Let's check how Y is derived. Usually it's HashToCurve(Secret).
			// Looking at api/cashu/types.go or pkg/crypto.
			// pkg/crypto/bdhke.go usually has HashToCurve.
			// I'll use a placeholder/helper if I can't find it immediately,
			// but I should check.
			// Wait, the proofs table has a Y column. I should calculate it.
			Y, err := crypto.HashToCurve([]byte(secrets[i]))
			if err == nil {
				(*availableProofs)[len(*availableProofs)-1].Y = cashu.WrappedPublicKey{PublicKey: Y}
			}
			(*availableProofs)[len(*availableProofs)-1].SeenAt = timestamp.Unix()
			(*availableProofs)[len(*availableProofs)-1].State = cashu.PROOF_UNSPENT
			(*availableProofs)[len(*availableProofs)-1].Quote = &quoteId
		}

		// Save Blind Signatures (Recovery)
		if err := db.SaveRestoreSigs(tx, recoverSigs); err != nil {
			log.Printf("Mint: Failed to save restore sigs: %v", err)
			return
		}

		// We DO NOT save the proofs here for minting?
		// Usually the mint doesn't store the proofs until they are spent?
		// Wait, the mint stores used proofs (nullifiers/Y).
		// So for MINT operation, we don't insert into `proofs` table.
		// `proofs` table tracks SPENT proofs (nullifiers).
	}

	if err := db.Commit(ctx, tx); err != nil {
		log.Printf("Mint: Failed to commit: %v", err)
	}
}

func processMelt(ctx context.Context, db postgresql.Postgresql, timestamp time.Time, availableProofs *[]cashu.Proof) {
	if len(*availableProofs) == 0 {
		return
	}

	quoteId, _ := generateRandomHex(16)
	targetAmount := uint64((randomInt(50) + 1) * 1000)

	stateChoice := randomInt(2) // 0: UNPAID, 1: PAID
	var state cashu.ACTION_STATE
	var paid bool
	var feePaid uint64
	var preimage string

	switch stateChoice {
	case 0:
		state = cashu.UNPAID
		paid = false
	case 1:
		state = cashu.PAID
		paid = true
		feePaid = 100 // Dummy fee
		preimage = "preimage_" + quoteId
	}

	// Select proofs
	var selectedProofs []cashu.Proof
	var selectedAmount uint64
	var indicesToRemove []int

	// Simple selection strategy
	for i, p := range *availableProofs {
		if selectedAmount >= targetAmount+feePaid {
			break
		}
		selectedProofs = append(selectedProofs, p)
		selectedAmount += p.Amount
		indicesToRemove = append(indicesToRemove, i)
	}

	if paid && selectedAmount < targetAmount+feePaid {
		// Not enough funds, degrade to unpaid or skip
		state = cashu.UNPAID
		paid = false
		selectedProofs = nil
	}

	actualAmount := targetAmount
	if !paid {
		actualAmount = targetAmount // Just the request amount
	}

	req := cashu.MeltRequestDB{
		Quote:           quoteId,
		Request:         "lnbcrt" + quoteId,
		Amount:          actualAmount,
		FeeReserve:      feePaid * 2, // Dummy reserve
		Expiry:          timestamp.Add(time.Hour * 24).Unix(),
		Unit:            cashu.Sat.String(),
		RequestPaid:     paid,
		Melted:          paid,
		State:           state,
		PaymentPreimage: preimage,
		SeenAt:          timestamp.Unix(),
		FeePaid:         feePaid,
	}

	tx, err := db.GetTx(ctx)
	if err != nil {
		log.Printf("Melt: Failed to get tx: %v", err)
		return
	}
	defer db.Rollback(ctx, tx)

	if err := db.SaveMeltRequest(tx, req); err != nil {
		log.Printf("Melt: Failed to save request: %v", err)
		return
	}

	if paid {
		// Mark proofs as spent in DB
		// The `SaveProof` function in backend seems to insert proofs.
		// In Nutmix, `proofs` table is likely for spent proofs (nullifiers).
		// Let's verify this assumption.
		// backend.go: GetProofsFromSecret -> SELECT ... FROM proofs
		// If it's for double spending check, then inserting means "spending".

		for i := range selectedProofs {
			selectedProofs[i].SeenAt = timestamp.Unix()
			selectedProofs[i].Quote = &quoteId
			selectedProofs[i].State = cashu.PROOF_SPENT
		}

		if err := db.SaveProof(tx, selectedProofs); err != nil {
			log.Printf("Melt: Failed to save proofs (spend): %v", err)
			return
		}

		// Remove from available
		// Need to remove indices carefully (reverse order)
		for i := len(indicesToRemove) - 1; i >= 0; i-- {
			idx := indicesToRemove[i]
			*availableProofs = append((*availableProofs)[:idx], (*availableProofs)[idx+1:]...)
		}
	}

	if err := db.Commit(ctx, tx); err != nil {
		log.Printf("Melt: Failed to commit: %v", err)
	}
}

// Helpers

func generateRandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func createBlindedMessages(amount uint64, keysetId string) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		// generate new private key r
		r, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		var B_ *secp256k1.PublicKey
		var secret string
		// generate random secret until it finds valid point
		for {
			secretBytes := make([]byte, 32)
			_, err = rand.Read(secretBytes)
			if err != nil {
				return nil, nil, nil, err
			}
			secret = hex.EncodeToString(secretBytes)
			B_, r, err = crypto.BlindMessage(secret, r)
			if err == nil {
				break
			}
		}

		blindedMessage := cashu.BlindedMessage{
			Amount: amt,
			B_:     cashu.WrappedPublicKey{PublicKey: B_},
			Id:     keysetId,
		}
		blindedMessages[i] = blindedMessage
		secrets[i] = secret
		rs[i] = r
	}

	return blindedMessages, secrets, rs, nil
}
