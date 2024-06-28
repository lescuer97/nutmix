package routes

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
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/mint"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
)

func v1bolt11Routes(r *gin.Engine, pool *pgxpool.Pool, mint mint.Mint) {
	v1 := r.Group("/v1")

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

		expireTime := cashu.ExpiryTime()

		switch lightningBackendType {
		case comms.FAKE_WALLET:
			payReq, err := lightning.CreateMockInvoice(mintRequest.Amount, "mock invoice", mint.Network, expireTime)
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
				Expiry:      expireTime,
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
				Expiry:      expireTime,
				Unit:        mintRequest.Unit,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		err = database.SaveQuoteMintRequest(pool, response)

		if err != nil {
			log.Println(fmt.Errorf("SaveQuoteRequest: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := database.GetMintQuoteById(pool, quoteId)
		if err != nil {
			log.Println(fmt.Errorf("GetMintQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		ok, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMintPayStatus)

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

		quote, err := database.GetMintQuoteById(pool, mintRequest.Quote)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.Minted {
			c.JSON(400, "Quote already minted")
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
		}
		blindedSignatures := []cashu.BlindSignature{}
		recoverySigsDb := []cashu.RecoverSigDB{}
		lightningBackendType := os.Getenv("MINT_LIGHTNING_BACKEND")

		switch lightningBackendType {

		case comms.FAKE_WALLET:
			invoice, err := zpay32.Decode(quote.Request, &mint.Network)

			if err != nil {
				log.Println(fmt.Errorf("zpay32.Decode: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)

			if err != nil {
				log.Println(fmt.Errorf("UnmarshallAmt: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			// check the amount in outputs are the same as the quote
			if int32(*invoice.MilliSat) != int32(amountMilsats) {
				log.Println(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)))
				c.JSON(403, "Amounts in outputs are not the same")
				return
			}
			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {

				if errors.Is(err, m.ErrInvalidBlindMessage) {
					c.JSON(400, m.ErrInvalidBlindMessage.Error())
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
				err := database.ModifyQuoteMintPayStatus(pool, true, quote.Quote)
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

			amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)

			if err != nil {
				log.Println(fmt.Errorf("UnmarshallAmt: %w", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			// check the amount in outputs are the same as the quote
			if int32(*invoice.MilliSat) != int32(amountMilsats) {
				log.Println(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)))
				c.JSON(403, "Amounts in outputs are not the same")
				return
			}

			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {
				log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
				if errors.Is(err, m.ErrInvalidBlindMessage) {
					c.JSON(403, m.ErrInvalidBlindMessage.Error())
					return
				}

				c.JSON(500, "Opps!, something went wrong")
				return
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		quote.Minted = true

		err = database.ModifyQuoteMintMintedStatus(pool, quote.Minted, quote.Quote)

		if err != nil {
			log.Println(fmt.Errorf("ModifyQuoteMintMintedStatus: %w", err))
		}
		err = database.SetRestoreSigs(pool, recoverySigsDb)
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

		expireTime := cashu.ExpiryTime()

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
				Expiry:     expireTime,
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
			hexHash := hex.EncodeToString(invoice.PaymentHash[:])

			response = cashu.PostMeltQuoteBolt11Response{
				Paid:       false,
				Expiry:     expireTime,
				FeeReserve: (fee + 1),
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

		err = database.SaveQuoteMeltRequest(pool, dbRequest)

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

		quote, err := database.GetMeltQuoteById(pool, quoteId)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %w", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		ok, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMeltPayStatus)

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

		quote, err := database.GetMeltQuoteById(pool, meltRequest.Quote)

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

		if AmountProofs < quote.Amount+quote.FeeReserve {
			log.Printf("Not enought proofs to expend. Needs: %v", quote.Amount)
			c.JSON(403, "Not enought proofs to expend. Needs: %v")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := database.CheckListOfProofs(pool, CList, SecretList)

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
			case errors.Is(err, cashu.ErrNotEnoughSignatures):
				c.JSON(403, cashu.ErrNotEnoughSignatures.Error())
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
			payment, err := mint.LightningComs.PayInvoice(quote.Request, quote.FeeReserve)

			if err != nil {
				log.Printf("mint.LightningComs.PayInvoice %+v", err)
				c.JSON(400, "could not make payment")
				return
			}

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

			// if fees where lower than expected return sats to the user
			feesInSat := uint64(payment.PaymentRoute.TotalFeesMsat / 1000)

			if feesInSat < quote.FeeReserve && len(meltRequest.Outputs) > 0 {

				overpaidFees := quote.FeeReserve - feesInSat
				amounts := cashu.AmountSplit(overpaidFees)
				change := meltRequest.Outputs
				switch {
				case len(amounts) > len(meltRequest.Outputs):
					for i := range change {
						change[i].Amount = amounts[i]
					}

				default:
					change = change[:len(amounts)]

					for i := range change {
						change[i].Amount = amounts[i]
					}

				}
				blindSignatures, recoverySigsDb, err := mint.SignBlindedMessages(change, quote.Unit)

				if err != nil {
					log.Println(fmt.Errorf("mint.SignBlindedMessages: %w", err))
					c.JSON(500, "Opps!, something went wrong")
					return
				}

				err = database.SetRestoreSigs(pool, recoverySigsDb)

				if err != nil {
					log.Println(fmt.Errorf("SetRecoverySigs: %w", err))
					log.Println(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb))
				}

				response.Change = blindSignatures

			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		quote.Melted = true
		err = database.ModifyQuoteMeltPayStatusAndMelted(pool, quote.RequestPaid, quote.Melted, meltRequest.Quote)
		if err != nil {
			log.Println(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err))
			c.JSON(200, response)
			return
		}
		// send proofs to database
		err = database.SaveProofs(pool, meltRequest.Inputs)

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
}
