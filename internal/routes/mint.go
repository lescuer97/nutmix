package routes

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/tyler-smith/go-bip32"
)

func v1MintRoutes(ctx context.Context, r *gin.Engine, pool *pgxpool.Pool, mint mint.Mint) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keys := mint.OrderActiveKeysByUnit()

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")

		keysets, err := mint.GetKeysetById(id)

		if err != nil {
			log.Printf("GetKeysetById: %+v ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys := cashu.OrderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		seeds, err := database.GetAllSeeds(pool)
		if err != nil {
			c.JSON(500, "Server side error")
			return
		}

		keys := make(map[string][]cashu.BasicKeysetResponse)
		keys["keysets"] = []cashu.BasicKeysetResponse{}

		for _, seed := range seeds {
			keys["keysets"] = append(keys["keysets"], cashu.BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active})
		}

		c.JSON(200, keys)
	})

	v1.GET("/info", func(c *gin.Context) {

		seed, err := database.GetActiveSeed(pool)

		var pubkey string = ""

		if err != nil {
			c.JSON(500, "Server side error")
			return
		}

		masterKey, err := bip32.NewMasterKey(seed.Seed)

		if err != nil {
			log.Printf("Error creating master key: %v ", err)
			c.JSON(500, "Server side error")
			return
		}
		pubkey = hex.EncodeToString(masterKey.PublicKey().Key)
		name := os.Getenv("NAME")
		description := os.Getenv("DESCRIPTION")
		description_long := os.Getenv("DESCRIPTION_LONG")
		motd := os.Getenv("MOTD")

		email := []string{"email", os.Getenv("EMAIL")}
		nostr := []string{"nostr", os.Getenv("NOSTR")}

		contacts := [][]string{email, nostr}

		for i, contact := range contacts {
			if contact[1] == "" {
				contacts = append(contacts[:i], contacts[i+1:]...)
			}
		}

		nuts := make(map[string]cashu.SwapMintInfo)
		var activeNuts []string = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}

		for _, nut := range activeNuts {
			nuts[nut] = cashu.SwapMintInfo{
				Disabled: false,
			}
		}

		response := cashu.GetInfoResponse{
			Name:            name,
			Version:         "NutMix/0.1",
			Pubkey:          pubkey,
			Description:     description,
			DescriptionLong: description_long,
			Motd:            motd,
			Contact:         contacts,
			Nuts:            nuts,
		}

		c.JSON(200, response)
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := c.BindJSON(&swapRequest)
		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		var AmountProofs, AmountSignature uint64
		var CList, SecretsList []string

		if len(swapRequest.Inputs) == 0 || len(swapRequest.Outputs) == 0 {
			log.Println("Inputs or Outputs are empty")
			c.JSON(400, "Inputs or Outputs are empty")
			return
		}

		// check proof have the same amount as blindedSignatures
		for i, proof := range swapRequest.Inputs {
			AmountProofs += proof.Amount
			CList = append(CList, proof.C)
			SecretsList = append(SecretsList, proof.Secret)

			p, err := proof.HashSecretToCurve()

			if err != nil {
				log.Printf("proof.HashSecretToCurve(): %+v", err)
				c.JSON(400, "Problem processing proofs")
				return
			}
			swapRequest.Inputs[i] = p
		}

		for _, output := range swapRequest.Outputs {
			AmountSignature += output.Amount
		}

		if AmountProofs < AmountSignature {
			c.JSON(400, "Not enough proofs for signatures")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := database.CheckListOfProofs(pool, CList, SecretsList)

		if err != nil {
			log.Printf("CheckListOfProofs: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		if len(knownProofs) != 0 {
			log.Printf("Proofs already used: %+v", knownProofs)
			c.JSON(400, "Proofs already used")
			return
		}

		unit, err := mint.CheckProofsAreSameUnit(swapRequest.Inputs)

		if err != nil {
			log.Printf("CheckProofsAreSameUnit: %+v", err)
			c.JSON(400, "Proofs are not the same unit")
			return
		}
		err = mint.VerifyListOfProofs(swapRequest.Inputs, swapRequest.Outputs, unit)

		if err != nil {
			log.Println(fmt.Errorf("mint.VerifyListOfProofs: %w", err))

			switch {
			case errors.Is(err, cashu.ErrEmptyWitness):
				c.JSON(403, "Empty Witness")
				return
			case errors.Is(err, cashu.ErrNoValidSignatures):
				c.JSON(403, "No valid signatures")
				return
			case errors.Is(err, cashu.ErrNotEnoughSignatures):
				c.JSON(403, cashu.ErrNotEnoughSignatures.Error())
				return

			}

			c.JSON(403, "Invalid Proof")
			return
		}

		// sign the outputs
		blindedSignatures, recoverySigsDb, err := mint.SignBlindedMessages(swapRequest.Outputs, cashu.Sat.String())

		if err != nil {
			log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		response := cashu.PostSwapResponse{
			Signatures: blindedSignatures,
		}

		// send proofs to database
		err = database.SaveProofs(pool, swapRequest.Inputs)

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", swapRequest.Inputs))
			c.JSON(200, response)
			return
		}

		err = database.SetRestoreSigs(pool, recoverySigsDb)
		if err != nil {
			log.Println(fmt.Errorf("SetRecoverySigs: %w", err))
			log.Println(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb))
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

	v1.POST("/checkstate", func(c *gin.Context) {
		var checkStateRequest cashu.PostCheckStateRequest
		err := c.BindJSON(&checkStateRequest)
		if err != nil {
			log.Println(fmt.Errorf("c.BindJSON: %w", err))
			c.JSON(400, "Malformed Body")
			return
		}

		checkStateResponse := cashu.PostCheckStateResponse{
			States: make([]cashu.CheckState, 0),
		}

		// set as unspent
		proofs, err := database.CheckListOfProofsBySecretCurve(pool, checkStateRequest.Ys)

		proofsForRemoval := make([]cashu.Proof, 0)

		for _, state := range checkStateRequest.Ys {

			pendingAndSpent := false

			checkState := cashu.CheckState{
				Y:       state,
				State:   cashu.PROOF_UNSPENT,
				Witness: nil,
			}

			switch {
			// check if is in list of pending proofs
			case slices.ContainsFunc(mint.PendingProofs, func(p cashu.Proof) bool {
				checkState.Witness = &p.Witness
				return p.Y == state
			}):
				pendingAndSpent = true
				checkState.State = cashu.PROOF_PENDING
			// Check if is in list of spents and if its also pending add it for removal of pending list
			case slices.ContainsFunc(proofs, func(p cashu.Proof) bool {
				compare := p.Y == state
				checkState.Witness = &p.Witness
				if compare && pendingAndSpent {

					proofsForRemoval = append(proofsForRemoval, p)
				}
				return compare
			}):
				checkState.State = cashu.PROOF_SPENT
			}

			checkStateResponse.States = append(checkStateResponse.States, checkState)
		}

		// remove proofs from pending proofs
		if len(proofsForRemoval) != 0 {
			newPendingProofs := []cashu.Proof{}
			for _, proof := range mint.PendingProofs {
				if !slices.Contains(proofsForRemoval, proof) {
					newPendingProofs = append(newPendingProofs, proof)
				}
			}
		}

		c.JSON(200, checkStateResponse)

	})
	v1.POST("/restore", func(c *gin.Context) {
		var restoreRequest cashu.PostRestoreRequest
		err := c.BindJSON(&restoreRequest)

		if err != nil {
			log.Println(fmt.Errorf("c.BindJSON: %w", err))
			c.JSON(400, "Malformed body request")
			return
		}

		blindingFactors := []string{}

		for _, output := range restoreRequest.Outputs {
			blindingFactors = append(blindingFactors, output.B_)
		}

		blindRecoverySigs, err := database.GetRestoreSigsFromBlindedMessages(pool, blindingFactors)
		if err != nil {
			log.Println(fmt.Errorf("GetRestoreSigsFromBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		restoredBlindSigs := []cashu.BlindSignature{}
		restoredBlindMessage := []cashu.BlindedMessage{}

		for _, sigRecover := range blindRecoverySigs {
			restoredSig, restoredMessage := sigRecover.GetSigAndMessage()
			restoredBlindSigs = append(restoredBlindSigs, restoredSig)
			restoredBlindMessage = append(restoredBlindMessage, restoredMessage)
		}

		c.JSON(200, cashu.PostRestoreResponse{
			Outputs:    restoredBlindMessage,
			Signatures: restoredBlindSigs,
		})
	})

}
