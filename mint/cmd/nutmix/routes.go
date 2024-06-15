package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/invoices"
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
		var activeNuts []string = []string{"1", "2", "3", "4", "5", "6"}

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

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		err := c.BindJSON(&mintRequest)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

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
				Quote:       randUuid.String(),
				Request:     payReq,
				RequestPaid: true,
				Expiry:      cashu.ExpiryTime,
				Unit:        mintRequest.Unit,
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
				Quote:       hash,
				Request:     resInvoice.GetPaymentRequest(),
				RequestPaid: false,
				Expiry:      cashu.ExpiryTime,
				Unit:        mintRequest.Unit,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		err = SaveQuoteMintRequest(pool, response)

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
			log.Println(fmt.Errorf("GetMintQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		ok, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, ModifyQuoteMintPayStatus)

		if err != nil {
			log.Println(fmt.Errorf("VerifyLightingPaymentHappened: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		quote.RequestPaid = ok

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

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.Minted {
			c.JSON(400, "Quote already minted")
			return
		}

		blindedSignatures := []cashu.BlindSignature{}

		recoverySigsDb := []cashu.RecoverSigDB{}
		lightningBackendType := os.Getenv("MINT_LIGHTNING_BACKEND")

		switch lightningBackendType {

		case comms.FAKE_WALLET:
			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {

				log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
				if errors.Is(err, ErrInvalidBlindMessage) {
					c.JSON(400, ErrInvalidBlindMessage.Error())
					return
				}

				c.JSON(500, "Opps!, something went wrong")
				return
			}
			// blindedSignatures = signedSignatures

		case comms.LND_WALLET:
			invoiceDB, err := mint.LightningComs.CheckIfInvoicePayed(quote.Quote)

			if err != nil {
				log.Println(fmt.Errorf("mint.LightningComs.CheckIfInvoicePayed: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			if invoiceDB.State == lnrpc.Invoice_SETTLED {
				err := ModifyQuoteMintPayStatus(pool, true, quote.Quote)
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

			var amount uint64 = 0

			for _, output := range mintRequest.Outputs {
				amount += output.Amount
			}

			amountMilsats, err := lnrpc.UnmarshallAmt(int64(amount), 0)

			if err != nil {
				log.Println(fmt.Errorf("UnmarshallAmt: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			// check the amount in outputs are the same as the quote
			if int32(*invoice.MilliSat) != int32(amountMilsats) {
				log.Println(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)))
				c.JSON(400, "Amounts in outputs are not the same")
				return
			}

			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {
				log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
				if errors.Is(err, ErrInvalidBlindMessage) {
					c.JSON(400, ErrInvalidBlindMessage.Error())
					return
				}

				c.JSON(500, "Opps!, something went wrong")
				return
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		quote.Minted = true

		err = ModifyQuoteMintMintedStatus(pool, quote.Minted, quote.Quote)

		if err != nil {
			log.Println(fmt.Errorf("ModifyQuoteMintMintedStatus: %w", err))
		}
		err = SetRestoreSigs(pool, recoverySigsDb)
		if err != nil {
			log.Println(fmt.Errorf("SetRecoverySigs: %w", err))
			log.Println(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb))
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
		err = SaveProofs(pool, swapRequest.Inputs)

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", swapRequest.Inputs))
			c.JSON(200, response)
			return
		}

		err = SetRestoreSigs(pool, recoverySigsDb)
		if err != nil {
			log.Println(fmt.Errorf("SetRecoverySigs: %w", err))
			log.Println(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb))
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

	v1.POST("/melt/quote/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltQuoteBolt11Request
		err := c.BindJSON(&meltRequest)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		invoice, err := zpay32.Decode(meltRequest.Request, &mint.Network)
		// hash := hex.EncodeToString(*invoice.PaymentHash[:])
		if err != nil {
			log.Println(fmt.Errorf("zpay32.Decode: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		response := cashu.PostMeltQuoteBolt11Response{}
		dbRequest := cashu.MeltRequestDB{}

		lightningBackendType := os.Getenv("MINT_LIGHTNING_BACKEND")
		switch lightningBackendType {
		case comms.FAKE_WALLET:

			randUuid, err := uuid.NewRandom()

			if err != nil {
				log.Println(fmt.Errorf("NewRamdom: %w", err))

				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMeltQuoteBolt11Response{
				Paid:       true,
				Expiry:     cashu.ExpiryTime,
				FeeReserve: 1,
				Amount:     uint64(*invoice.MilliSat) / 1000,
				Quote:      randUuid.String(),
			}

			dbRequest = cashu.MeltRequestDB{
				Quote:       response.Quote,
				Request:     meltRequest.Request,
				Unit:        cashu.Sat.String(),
				Expiry:      response.Expiry,
				Amount:      response.Amount,
				FeeReserve:  response.FeeReserve,
				RequestPaid: response.Paid,
			}

		case comms.LND_WALLET:
			query, err := mint.LightningComs.QueryPayment(invoice)

			if err != nil {
				log.Println(fmt.Errorf("mint.LightningComs.PayInvoice: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			fee := lightning.GetAverageRouteFee(query.Routes) / 1000
			// randUuid, err := uuid.NewRandom()
			hexHash := hex.EncodeToString(invoice.PaymentHash[:])

			if err != nil {
				log.Println(fmt.Errorf("NewRamdom: %w", err))

				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMeltQuoteBolt11Response{
				Paid:       false,
				Expiry:     cashu.ExpiryTime,
				FeeReserve: fee,
				Amount:     uint64(*invoice.MilliSat) / 1000,
				Quote:      hexHash,
			}

			dbRequest = cashu.MeltRequestDB{
				Quote:       response.Quote,
				Request:     meltRequest.Request,
				Unit:        cashu.Sat.String(),
				Expiry:      response.Expiry,
				Amount:      response.Amount,
				FeeReserve:  response.FeeReserve,
				RequestPaid: response.Paid,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
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

		ok, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, ModifyQuoteMeltPayStatus)

		if err != nil {
			if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
				c.JSON(200, quote.GetPostMeltQuoteResponse())
				return
			}
			log.Println(fmt.Errorf("VerifyLightingPaymentHappened: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		quote.RequestPaid = ok

		c.JSON(200, quote.GetPostMeltQuoteResponse())
	})

	v1.POST("/melt/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltBolt11Request
		err := c.BindJSON(&meltRequest)
		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		if len(meltRequest.Inputs) == 0 {
			c.JSON(400, "Outputs are empty")
			return
		}

		quote, err := GetMeltQuoteById(pool, meltRequest.Quote)

		if err != nil {
			log.Println(fmt.Errorf("GetMeltQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		if quote.Melted {
			c.JSON(400, "Quote already melted")
			return
		}

		var CList, SecretList []string
		var AmountProofs uint64

		// check proof have the same amount as blindedSignatures
		for i, proof := range meltRequest.Inputs {
			AmountProofs += proof.Amount
			CList = append(CList, proof.C)
			SecretList = append(SecretList, proof.Secret)

			p, err := proof.HashSecretToCurve()

			if err != nil {
				log.Printf("proof.HashSecretToCurve(): %+v", err)
				c.JSON(400, "Problem processing proofs")
				return
			}

			meltRequest.Inputs[i] = p
			mint.PendingProofs = append(mint.PendingProofs, p)

		}

		if AmountProofs < quote.Amount {
			log.Printf("Not enought proofs to expend. Needs: %v", quote.Amount)
			c.JSON(403, "Not enought proofs to expend. Needs: %v")
			return
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

		unit, err := mint.CheckProofsAreSameUnit(meltRequest.Inputs)

		if err != nil {
			log.Printf("CheckProofsAreSameUnit: %+v", err)
			c.JSON(400, "Proofs are not the same unit")
			return
		}

		err = mint.VerifyListOfProofs(meltRequest.Inputs, []cashu.BlindedMessage{}, unit)

		if err != nil {
			log.Println(fmt.Errorf("mint.VerifyListOfProofs: %w", err))

			switch {
			case errors.Is(err, cashu.ErrEmptyWitness):
				c.JSON(403, "Empty Witness")
				return
			case errors.Is(err, cashu.ErrNoValidSignatures):
				c.JSON(403, "No valid signatures")
				return

			}

			c.JSON(403, "Invalid Proof")
			return
		}

		response := cashu.PostMeltBolt11Response{}

		lightningBackendType := os.Getenv("MINT_LIGHTNING_BACKEND")
		switch lightningBackendType {
		case comms.FAKE_WALLET:
			quote.RequestPaid = true
			response = cashu.PostMeltBolt11Response{
				Paid:            true,
				PaymentPreimage: "MockPaymentPreimage",
			}

		case comms.LND_WALLET:
			payment, err := mint.LightningComs.PayInvoice(quote.Request)

			if err != nil {
				log.Printf("mint.LightningComs.PayInvoice %+v", err)
				c.JSON(400, "could not make payment")
				return
			}

			// catch the comm
			switch {
			case payment.PaymentError == "invoice is already paid":
				c.JSON(400, "invoice is already paid")
				return
			case payment.PaymentError == "unable to find a path to destination":
				c.JSON(400, "unable to find a path to destination")
				return
			case payment.PaymentError != "":
				log.Printf("unknown lighting error: %+v", payment.PaymentError)
				c.JSON(500, "Unknown error happend while paying")
				return

			}

			quote.RequestPaid = true
			response = cashu.PostMeltBolt11Response{
				Paid:            true,
				PaymentPreimage: string(payment.PaymentPreimage),
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		quote.Melted = true
		err = ModifyQuoteMeltPayStatusAndMelted(pool, quote.RequestPaid, quote.Melted, meltRequest.Quote)
		if err != nil {
			log.Println(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err))
			c.JSON(200, response)
			return
		}
		// send proofs to database
		err = SaveProofs(pool, meltRequest.Inputs)

		if err != nil {
			log.Println(fmt.Errorf("SaveProofs: %w", err))
			log.Println(fmt.Errorf("Proofs: %+v", meltRequest.Inputs))
			c.JSON(200, response)
			return
		}

		newPendingProofs := []cashu.Proof{}
		// remove proofs from pending proofs
		for _, proof := range mint.PendingProofs {
			if !slices.Contains(meltRequest.Inputs, proof) {
				newPendingProofs = append(newPendingProofs, proof)
			}
		}

		mint.PendingProofs = newPendingProofs

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
		proofs, err := CheckListOfProofsBySecretCurve(pool, checkStateRequest.Ys)

		proofsForRemoval := make([]cashu.Proof, 0)

		for _, state := range checkStateRequest.Ys {

			pendingAndSpent := false

			checkState := cashu.CheckState{
				Y:       state,
				State:   cashu.UNSPENT,
				Witness: nil,
			}

			switch {
			// check if is in list of pending proofs
			case slices.ContainsFunc(mint.PendingProofs, func(p cashu.Proof) bool {
				checkState.Witness = &p.Witness
				return p.Y == state
			}):
				pendingAndSpent = true
				checkState.State = cashu.PENDING
			// Check if is in list of spents and if its also pending add it for removal of pending list
			case slices.ContainsFunc(proofs, func(p cashu.Proof) bool {
				compare := p.Y == state
				checkState.Witness = &p.Witness
				if compare && pendingAndSpent {

					proofsForRemoval = append(proofsForRemoval, p)
				}
				return compare
			}):
				checkState.State = cashu.SPENT
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

		blindRecoverySigs, err := GetRestoreSigsFromBlindedMessages(pool, blindingFactors)
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
