package main

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/cashu"
	"github.com/lescuer97/nutmix/crypto"
	"github.com/lescuer97/nutmix/lightning"
	"github.com/lightningnetwork/lnd/channeldb/migration_01_to_11/zpay32"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/tyler-smith/go-bip32"
	"log"
	"os"
)

func V1Routes(r *gin.Engine, conn *pgx.Conn, mint Mint) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keys := mint.OrderActiveKeysByUnit()

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")

		keysets, err := mint.GetKeysetById(cashu.Sats, id)

		if err != nil {
			log.Printf("GetKeysetById: %+v ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys := cashu.OrderKeysetByUnit(keysets)

		if err != nil {
			log.Printf("orderKeysetByUnit: %+v", err)
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		seeds, err := GetAllSeeds(conn)
		if err != nil {
			log.Fatalf("GetAllSeeds: %+v ", err)
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

		seed, err := GetActiveSeed(conn)

		var pubkey string = ""

		if err != nil {
			log.Fatal("Error getting active seed: ", err)
		}

		masterKey, err := bip32.NewMasterKey(seed.Seed)

		if err != nil {
			log.Fatal("Error creating master key ", err)
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

		nuts["1"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		nuts["2"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		nuts["6"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		response := cashu.GetInfoResponse{
			Name:            name,
			Version:         "AwesomeGoMint/0.1",
			Pubkey:          pubkey,
			Description:     description,
			DescriptionLong: description_long,
			Motd:            motd,
			Contact:         contacts,
			Nuts:            nuts,
		}

		c.JSON(200, response)
	})

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		c.BindJSON(&mintRequest)

		if mintRequest.Amount == 0 {
			c.JSON(400, "amount missing")
			return
		}

		payReq, err := lightning.CreateMockInvoice(mintRequest.Amount, "mock invoice")
		if err != nil {
			log.Println(err)
			c.JSON(500, "Opps!, something went wrong")

		}

		randUuid, err := uuid.NewRandom()

		if err != nil {
			log.Println(fmt.Errorf("NewRamdom: %w", err))

			c.JSON(500, "Opps!, something went wrong")
		}

		postRequest := cashu.PostMintQuoteBolt11Response{
			Quote:   randUuid.String(),
			Request: payReq,
			Paid:    true,
			Expiry:  3600,
		}

		err = SaveQuoteRequest(conn, postRequest)
		if err != nil {
			log.Println(fmt.Errorf("SaveQuoteRequest: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		c.JSON(200, postRequest)
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := GetQuoteById(conn, quoteId)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		c.JSON(200, quote)
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
		}

		quote, err := GetQuoteById(conn, mintRequest.Quote)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			if err == pgx.ErrNoRows {
				c.JSON(404, "Quote not found")
				return

			}
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		// check if the quote is paid
		if quote.Paid == false {
			c.JSON(400, "Quote not paid")
			return
		}

		invoice, err := zpay32.Decode(quote.Request, &chaincfg.MainNetParams)

		if err != nil {
			log.Println(fmt.Errorf("zpay32.Decode: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		var amount int32 = 0
		var amounts []int32 = make([]int32, 0)

		for _, output := range mintRequest.Outputs {
			amount += output.Amount
			amounts = append(amounts, output.Amount)
		}

		amountMilsats, err := lnrpc.UnmarshallAmt(int64(amount), 0)

		if err != nil {
			log.Println(fmt.Errorf("UnmarshallAmt: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) != int32(amountMilsats) {
			c.JSON(400, "Amounts in outputs are not the same")
			return
		}

		blindedSignatures, err := mint.SignBlindedMessages(mintRequest.Outputs, cashu.Sats)
		if err != nil {
			log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		// Store BlidedSignature
		c.JSON(200, cashu.PostMintBolt11Response{
			Signatures: blindedSignatures,
		})
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := c.BindJSON(&swapRequest)

		var AmountProofs, AmountSignature int32
		var CList, SecretList []string

		if len(swapRequest.Inputs) == 0 || len(swapRequest.Outputs) == 0 {
			c.JSON(400, "Inputs or Outputs are empty")
			return
		}
		// check proof have the same amount as blindedSignatures
		for _, proof := range swapRequest.Inputs {
			AmountProofs += proof.Amount
			CList = append(CList, proof.C)
			SecretList = append(SecretList, proof.Secret)
		}
		for _, output := range swapRequest.Outputs {
			AmountSignature += output.Amount
		}

		if AmountProofs != AmountSignature {
			c.JSON(400, "Amounts in proofs are not the same as in signatures")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := CheckListOfProofs(conn, CList, SecretList)

		fmt.Printf("knownProofs: %v\n", knownProofs)

		if err != nil {
			log.Printf("CheckListOfProofs: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		if len(knownProofs) != 0 {
			c.JSON(400, "Proofs already used")
			return
		}

		fmt.Printf("swapRequest.Inputs: %v\n", swapRequest.Inputs)

		// verify the proofs signatures are correct
		for _, proof := range swapRequest.Inputs {

			var keysetToUse cashu.Keyset
			for _, keyset := range mint.Keysets[cashu.Sats] {
				if keyset.Amount == int(proof.Amount) && keyset.Id == proof.Id {
					keysetToUse = keyset
					break
				}
			}

			// check if keysetToUse is not assigned
			if keysetToUse.Id == "" {
				c.JSON(500, "Proofs id not found in the database")
				return
			}

			fmt.Printf("keysetToUse: %v\n", keysetToUse)

			parsedBlinding, err := hex.DecodeString(proof.C)

			if err != nil {
				log.Printf("hex.DecodeString: %+v", err)
				c.JSON(400, "could not decode a proof")
				return
			}

			pubkey, err := secp256k1.ParsePubKey(parsedBlinding)
			if err != nil {
				log.Printf("secp256k1.ParsePubKey: %+v", err)
				c.JSON(400, "could not parse proof blinding factor")
				return
			}

			verified := crypto.Verify(proof.Secret, keysetToUse.PrivKey, pubkey)

			if !verified {
				c.JSON(403, "invalid proof")
				return
			}

		}

		// sign the outputs
		blindedSignatures, err := mint.SignBlindedMessages(swapRequest.Outputs, cashu.Sats)

		if err != nil {
			log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		response := cashu.PostSwapResponse{
			Signatures: blindedSignatures,
		}

		// send proofs to database
		err = SaveProofs(conn, swapRequest.Inputs)

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", swapRequest.Inputs))
			c.JSON(200, response)
		}

		c.JSON(200, response)
	})

}
