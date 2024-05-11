package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/cashu"
	"github.com/lescuer97/nutmix/comms"
	"github.com/lescuer97/nutmix/lightning"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/tyler-smith/go-bip32"
)

func V1Routes(r *gin.Engine, pool *pgxpool.Pool, mint Mint) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keys := mint.OrderActiveKeysByUnit()

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")

		keysets, err := mint.GetKeysetById(cashu.Sat.String(), id)

		if err != nil {
			log.Printf("GetKeysetById: %+v ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys := cashu.OrderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		seeds, err := GetAllSeeds(pool)
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

		seed, err := GetActiveSeed(pool)

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

		lightningBackendType := os.Getenv("MINT_LIGHTNING_BACKEND")

		var response cashu.PostMintQuoteBolt11Response

		switch lightningBackendType {
		case comms.FAKE_WALLET:
			payReq, err := lightning.CreateMockInvoice(mintRequest.Amount, "mock invoice", mint.Network)
			if err != nil {
				log.Println(err)
				c.JSON(500, "Opps!, something went wrong")
				return
			}
			randUuid, err := uuid.NewRandom()

			if err != nil {
				log.Println(fmt.Errorf("NewRamdom: %w", err))

				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMintQuoteBolt11Response{
				Quote:   randUuid.String(),
				Request: payReq,
				Paid:    true,
				Expiry:  cashu.ExpiryTime,
			}

		case comms.LND_WALLET:
			resInvoice, err := mint.LightningComs.RequestInvoice(mintRequest.Amount)

			if err != nil {
				log.Println(err)
				c.JSON(500, "Opps!, something went wrong")
				return

			}
			hash := hex.EncodeToString(resInvoice.GetRHash())
			response = cashu.PostMintQuoteBolt11Response{
				Quote:   hash,
				Request: resInvoice.GetPaymentRequest(),
				Paid:    false,
				Expiry:  cashu.ExpiryTime,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		err := SaveQuoteMintRequest(pool, response)
		if err != nil {
			log.Println(fmt.Errorf("SaveQuoteRequest: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := GetMintQuoteById(pool, quoteId)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, quote)
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		quote, err := GetMintQuoteById(pool, mintRequest.Quote)

		invoiceDB, err := mint.LightningComs.CheckIfInvoicePayed(quote.Quote)

		if invoiceDB.State == lnrpc.Invoice_SETTLED {
			quote.Paid = true
			err := ModifyQuoteMintPayStatus(pool, quote)
			if err != nil {
				log.Println(fmt.Errorf("SaveQuoteRequest: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

		} else {
			c.JSON(400, "Quote not paid")
			return
		}

		invoice, err := zpay32.Decode(quote.Request, &mint.Network)

		if err != nil {
			log.Println(fmt.Errorf("zpay32.Decode: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
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
			return
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) != int32(amountMilsats) {
			c.JSON(400, "Amounts in outputs are not the same")
			return
		}

		blindedSignatures, err := mint.SignBlindedMessages(mintRequest.Outputs, cashu.Sat.String())
		if err != nil {
			log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
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
		var CList, SecretsList []string

		if len(swapRequest.Inputs) == 0 || len(swapRequest.Outputs) == 0 {
			log.Println("Inputs or Outputs are empty")
			c.JSON(400, "Inputs or Outputs are empty")
			return
		}

		// check proof have the same amount as blindedSignatures
		for _, proof := range swapRequest.Inputs {
			AmountProofs += proof.Amount
			CList = append(CList, proof.C)
			SecretsList = append(SecretsList, proof.Secret)
		}

		for _, output := range swapRequest.Outputs {
			AmountSignature += output.Amount
		}

		if AmountProofs != AmountSignature {
			c.JSON(400, "Amounts in proofs are not the same as in signatures")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := CheckListOfProofs(pool, CList, SecretsList)

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

		// verify the proofs signatures are correct
		for _, proof := range swapRequest.Inputs {

			err := mint.ValidateProof(proof)
			if err != nil {
				log.Println(fmt.Errorf("ValidateProof: %w", err))
				c.JSON(403, "Invalid Proof")
				return
			}

		}

		// sign the outputs
		blindedSignatures, err := mint.SignBlindedMessages(swapRequest.Outputs, cashu.Sat.String())

		if err != nil {
			log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		response := cashu.PostSwapResponse{
			Signatures: blindedSignatures,
		}

		// send proofs to database
		err = SaveProofs(pool, swapRequest.Inputs)

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", swapRequest.Inputs))
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

	v1.POST("/melt/quote/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltQuoteBolt11Request
		err := c.BindJSON(&meltRequest)

		invoice, err := zpay32.Decode(meltRequest.Request, &mint.Network)

		if err != nil {
			log.Println(fmt.Errorf("zpay32.Decode: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		query, err := mint.LightningComs.QueryPayment(invoice)

		if err != nil {
			log.Println(fmt.Errorf("mint.LightningComs.PayInvoice: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		fee := lightning.GetAverageRouteFee(query.Routes) / 1000
		randUuid, err := uuid.NewRandom()

		if err != nil {
			log.Println(fmt.Errorf("NewRamdom: %w", err))

			c.JSON(500, "Opps!, something went wrong")
			return
		}

		response := cashu.PostMeltQuoteBolt11Response{
			Paid:       false,
			Expiry:     cashu.ExpiryTime,
			FeeReserve: fee,
			Amount:     int64(*invoice.MilliSat) / 1000,
			Quote:      randUuid.String(),
		}

		dbRequest := cashu.MeltRequestDB{
			Quote:      response.Quote,
			Request:    meltRequest.Request,
			Unit:       cashu.Sat.String(),
			Expiry:     response.Expiry,
			Amount:     response.Amount,
			FeeReserve: response.FeeReserve,
			Paid:       response.Paid,
		}

		err = SaveQuoteMeltRequest(pool, dbRequest)

		if err != nil {
			log.Println(fmt.Errorf("SaveQuoteMeltRequest: %w", err))
			log.Println(fmt.Errorf("dbRequest: %+v", dbRequest))
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := GetMeltQuoteById(pool, quoteId)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, quote.GetPostMeltQuoteResponse())
	})

	v1.POST("/melt/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltBolt11Request
		err := c.BindJSON(&meltRequest)

		if len(meltRequest.Inputs) == 0 {
			c.JSON(400, "Outputs are empty")
			return
		}

		if err != nil {
			log.Println(fmt.Errorf("c.BindJSON: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		quote, err := GetMeltQuoteById(pool, meltRequest.Quote)

		if err != nil {
			log.Println(fmt.Errorf("GetMeltQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		var CList, SecretList []string
		var AmountProofs int32

		// check proof have the same amount as blindedSignatures
		for _, proof := range meltRequest.Inputs {
			AmountProofs += proof.Amount
			CList = append(CList, proof.C)
			SecretList = append(SecretList, proof.Secret)
		}

		// check if we know any of the proofs
		knownProofs, err := CheckListOfProofs(pool, CList, SecretList)

		if err != nil {
			log.Printf("CheckListOfProofs: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		if len(knownProofs) != 0 {
			c.JSON(400, "Proofs already used")
			return
		}

		// verify the proofs signatures are correct
		for _, proof := range meltRequest.Inputs {

			err := mint.ValidateProof(proof)
			if err != nil {
				c.JSON(403, "Invalid Proof")
				return
			}
		}

		payment, err := mint.LightningComs.PayInvoice(quote.Request)

		if err != nil {
			log.Printf("mint.LightningComs.PayInvoice %+v", err)
			c.JSON(400, "could not make payment")
			return
		}

		if payment.PaymentError == "invoice is already paid" {
			c.JSON(400, "invoice is already paid")
			return
		}

		// send proofs to database
		err = SaveProofs(pool, meltRequest.Inputs)

		response := cashu.PostMeltBolt11Response{
			Paid:            true,
			PaymentPreimage: string(payment.PaymentPreimage),
		}

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", meltRequest.Inputs))
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

}
