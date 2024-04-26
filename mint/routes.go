package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/cashu"
	"github.com/lescuer97/nutmix/lightning"
	"github.com/lightningnetwork/lnd/channeldb/migration_01_to_11/zpay32"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/tyler-smith/go-bip32"
)

func V1Routes(r *gin.Engine, conn *pgx.Conn) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keysets, err := CheckForActiveKeyset(conn)

		if err != nil {
			log.Fatalf("CheckForActiveKeyset: %w ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys, err := cashu.OrderKeysetByUnit(keysets)
		if err != nil {
			log.Printf("orderKeysetByUnit: %w ", err)
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")
		keysets, err := CheckForKeysetById(conn, id)
		if err != nil {
			log.Fatalf("CheckForKeysetById: %w ", err)
			c.JSON(500, "Server side error")
			return
		}
		keys, err := cashu.OrderKeysetByUnit(keysets)
		if err != nil {
			log.Printf("orderKeysetByUnit: %w ", err)
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		seeds, err := GetAllSeeds(conn)
		if err != nil {
			log.Fatalf("GetAllSeeds: %w ", err)
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

		fmt.Println("amount: ", mintRequest.Amount)
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
			log.Printf("Incorrect body: %w", err)
			c.JSON(400, "Malformed body request")
		}

		quote, err := GetQuoteById(conn, mintRequest.Quote)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
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

		keysets, err := GetKeysetsByAmountList(conn, amounts)

		if err != nil {
			log.Println(fmt.Errorf("GetKeysetsByAmountList: %w", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		var blindedSignatures []cashu.BlindSignature
		// Create blindSignature with diffie-hellman exchange
		for _, output := range mintRequest.Outputs {

			index := slices.Index(cashu.PosibleKeysetValues, int(output.Amount))

			if index == -1 {
				c.JSON(500, "Something happened")
				return
			}

			key, err := bip32.B58Deserialize(keysets[int(output.Amount)].PrivKey)
			if err != nil {
				log.Println(fmt.Errorf("bip32.B58Deserialize: %w", err))
				c.JSON(500, "Opps!, something went wrong")
			}

			blindSignature, err := cashu.GenerateBlindSignature(key, output)

			if err != nil {
				log.Println(fmt.Errorf("GenerateBlindSignature: %w", err))
				c.JSON(500, "Opps!, something went wrong")
			}

			blindedSignatures = append(blindedSignatures, blindSignature)

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

		// check proof have the same amount as blindedSignatures
		for _, proof := range swapRequest.Inputs {
			AmountProofs += proof.Amount
		}
		for _, output := range swapRequest.Outputs {
			AmountSignature += output.Amount
		}

		if err != nil {
			log.Printf("Incorrect body: %w", swapRequest)
			log.Printf("Incorrect body: %w", err)
			c.JSON(400, "Malformed body request")
		}

		c.JSON(500, "OK")
	})

}
