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
	"github.com/joho/godotenv"
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

	// get variables from env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Failed to load env file: %v", err)
	}
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

	// 2. Main Loop - 24 Months, Weekly Requests
	// Track: Blind Signature and Proof Invariants
	// - Blind signatures are created in mint operations (one per blinded message)
	// - Proofs are created from blind signatures in mint operations (one per blind signature)
	// - Proofs are consumed (spent) in melt operations for paid requests
	// - Invariant: len(availableProofs) <= total_blind_signatures_created
	// - Invariant: proofs_saved_for_melt <= len(availableProofs) <= total_blind_signatures_created
	now := time.Now()
	startTime := now.AddDate(0, -24, 0)

	availableProofs := make([]cashu.Proof, 0)
	var currentKeyset map[uint64]cashu.MintKey
	var currentKeysetId string
	var currentMonth int = -1

	// Calculate total days: 24 months ≈ 730 days
	// Generate requests once per day (multiple times per week)
	totalDays := 24 * 30 // Approximate 30 days per month

	for day := 0; day < totalDays; day++ {
		currentDayTime := startTime.AddDate(0, 0, day)
		currentDayMonth := int(currentDayTime.Month()) - 1 + (currentDayTime.Year()-startTime.Year())*12

		// Rotate keyset monthly
		if currentDayMonth != currentMonth {
			currentMonth = currentDayMonth
			log.Printf("Processing month: %s", currentDayTime.Format("2006-01"))

			// --- Keyset Rotation ---
			fee := uint((currentMonth + 1) * 100)

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
				CreatedAt:   currentDayTime.Unix(),
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
			currentKeysetId = keysetId
		}

		// --- Request Generation - Weekly (multiple times per week) ---
		// Generate requests once per day (7 times per week)
		// Randomly choose Mint or Melt
		isMint := coinFlip()

		if isMint {
			processMint(ctx, db, currentDayTime, currentKeyset, currentKeysetId, &availableProofs)
		} else {
			processMelt(ctx, db, currentDayTime, &availableProofs)
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

		// Validate that blinded messages sum equals invoice amount
		var blindedMessagesSum uint64
		for _, msg := range blindedMessages {
			blindedMessagesSum += msg.Amount
		}
		if blindedMessagesSum != amount {
			log.Printf("Mint: Blinded messages sum (%d) does not match invoice amount (%d)", blindedMessagesSum, amount)
			return
		}

		// Sign Messages
		// Track: We create one blind signature per blinded message
		// This ensures blind signatures count matches blinded messages count
		var signatures []cashu.BlindSignature
		var recoverSigs []cashu.RecoverSigDB
		proofsBeforeMint := len(*availableProofs)

		for i, msg := range blindedMessages {
			key, ok := keyset[msg.Amount]
			if !ok {
				log.Printf("Mint: Key not found for amount %d", msg.Amount)
				return
			}

			// Sign - creates one blind signature per blinded message
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
			// Track: We create one proof per blind signature
			// This ensures proofs count matches blind signatures count
			C := crypto.UnblindSignature(C_, rs[i], key.PrivKey.PubKey())
			*availableProofs = append(*availableProofs, cashu.Proof{
				Amount: msg.Amount,
				Id:     keysetId,
				Secret: secrets[i],
				C:      cashu.WrappedPublicKey{PublicKey: C},
			})

			// Calculate Y for the proof (needed for DB SaveProof)
			Y, err := crypto.HashToCurve([]byte(secrets[i]))
			if err == nil {
				(*availableProofs)[len(*availableProofs)-1].Y = cashu.WrappedPublicKey{PublicKey: Y}
			}
			(*availableProofs)[len(*availableProofs)-1].SeenAt = timestamp.Unix()
			(*availableProofs)[len(*availableProofs)-1].State = cashu.PROOF_UNSPENT
			(*availableProofs)[len(*availableProofs)-1].Quote = &quoteId
		}

		// Validate: Number of blind signatures should equal number of blinded messages
		if len(signatures) != len(blindedMessages) {
			log.Printf("Mint: Number of blind signatures (%d) does not match blinded messages (%d)", len(signatures), len(blindedMessages))
			return
		}

		// Validate: Number of proofs created should equal number of blind signatures
		proofsCreated := len(*availableProofs) - proofsBeforeMint
		if proofsCreated != len(signatures) {
			log.Printf("Mint: Number of proofs created (%d) does not match blind signatures (%d)", proofsCreated, len(signatures))
			return
		}

		// Save Blind Signatures (Recovery)
		// Track: These blind signatures are saved to the database
		// The proofs created above come from these blind signatures
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

	// Select proofs to exactly match invoice amount + fees
	// Track: Only select proofs for paid melt requests
	// Ensure proofs don't exceed available blind signatures (they come from availableProofs)
	var selectedProofs []cashu.Proof
	var selectedAmount uint64
	var indicesToRemove []int
	requiredAmount := targetAmount + feePaid

	// Selection strategy: Select proofs that sum to exactly invoice amount + fees
	// Since proofs come in binary denominations, we select until we have enough
	// but try to minimize excess
	for i, p := range *availableProofs {
		// Stop if we have enough
		if selectedAmount >= requiredAmount {
			break
		}
		// Add proof if adding it doesn't exceed by too much
		// (with binary denominations, some excess is expected)
		newAmount := selectedAmount + p.Amount
		if newAmount <= requiredAmount || (selectedAmount < requiredAmount && newAmount-requiredAmount <= requiredAmount/10) {
			selectedProofs = append(selectedProofs, p)
			selectedAmount += p.Amount
			indicesToRemove = append(indicesToRemove, i)
		}
	}

	// Validate: For paid melt requests, we must have enough proofs
	if paid && selectedAmount < requiredAmount {
		// Not enough funds, degrade to unpaid
		state = cashu.UNPAID
		paid = false
		selectedProofs = nil
		indicesToRemove = nil
		selectedAmount = 0
	}

	// Track: Validate that proofs don't exceed available blind signatures
	// Since proofs come from availableProofs (which are created from blind signatures),
	// this invariant is maintained as long as we only use proofs from availableProofs
	if len(selectedProofs) > len(*availableProofs) {
		log.Printf("Melt: Selected proofs (%d) exceed available proofs (%d)", len(selectedProofs), len(*availableProofs))
		return
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
		// Track: Only save proofs for paid melt requests
		// The proofs selected above sum to invoice amount + fees (or as close as possible with binary denominations)
		// Validate: Ensure proofs don't exceed available blind signatures
		// Since proofs come from availableProofs (created from blind signatures in mint operations),
		// this invariant is maintained

		// Mark proofs as spent in DB
		// The `SaveProof` function inserts proofs into the `proofs` table.
		// In Nutmix, `proofs` table tracks spent proofs (nullifiers) for double-spend prevention.

		for i := range selectedProofs {
			selectedProofs[i].SeenAt = timestamp.Unix()
			selectedProofs[i].Quote = &quoteId
			selectedProofs[i].State = cashu.PROOF_SPENT
		}

		// Validate: Selected proofs amount should match invoice amount + fees (within binary denomination tolerance)
		if selectedAmount < requiredAmount {
			log.Printf("Melt: Selected proofs amount (%d) is less than required (%d)", selectedAmount, requiredAmount)
			return
		}

		if err := db.SaveProof(tx, selectedProofs); err != nil {
			log.Printf("Melt: Failed to save proofs (spend): %v", err)
			return
		}

		// Remove from available proofs
		// Track: This maintains the invariant that availableProofs only contains proofs
		// that haven't been spent yet (and thus correspond to available blind signatures)
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
