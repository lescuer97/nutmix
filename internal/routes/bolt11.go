package routes

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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
	"log"
	"log/slog"
	"slices"
	"strings"
)

func v1bolt11Routes(ctx context.Context, r *gin.Engine, pool *pgxpool.Pool, mint *mint.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		err := c.BindJSON(&mintRequest)

		if err != nil {
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}
		// TODO - REMOVE this when doing multi denomination tokens with Milisats
		if mintRequest.Unit != cashu.Sat.String() {
			log.Printf("Incorrect Unit for minting: %+v", mintRequest.Unit)
			c.JSON(400, "Incorrect Unit for minting")
			return
		}

		if mintRequest.Amount == 0 {
			c.JSON(400, "amount missing")
			return
		}

		lightningBackendType := ctx.Value("MINT_LIGHTNING_BACKEND").(string)

		var response cashu.PostMintQuoteBolt11Response

		expireTime := cashu.ExpiryTime()

		switch lightningBackendType {
		case comms.FAKE_WALLET:
			payReq, err := lightning.CreateMockInvoice(mintRequest.Amount, "mock invoice", mint.Network, expireTime)
			if err != nil {
				logger.Info(err.Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			randUuid, err := uuid.NewRandom()

			if err != nil {
				logger.Info(fmt.Errorf("NewRamdom: %w", err).Error())

				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMintQuoteBolt11Response{
				Quote:       randUuid.String(),
				Request:     payReq,
				RequestPaid: true,
				Expiry:      expireTime,
				Unit:        mintRequest.Unit,
				State:       cashu.PAID,
			}

		case comms.LND_WALLET, comms.LNBITS_WALLET:
			resInvoice, err := mint.LightningComs.RequestInvoice(mintRequest.Amount)

			if err != nil {
				logger.Info(err.Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMintQuoteBolt11Response{
				Quote:       resInvoice.Rhash,
				Request:     resInvoice.PaymentRequest,
				RequestPaid: false,
				Expiry:      expireTime,
				Unit:        mintRequest.Unit,
				State:       cashu.UNPAID,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		err = database.SaveQuoteMintRequest(pool, response)

		if err != nil {
			logger.Error(fmt.Errorf("SaveQuoteRequest: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := database.GetMintQuoteById(pool, quoteId)

		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			c.JSON(200, quote)
			return
		}
		if err != nil {
			logger.Error(fmt.Errorf("GetMintQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		state, _, err := mint.VerifyLightingPaymentHappened(ctx, pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMintPayStatus)

		if err != nil {
			logger.Warn(fmt.Errorf("VerifyLightingPaymentHappened: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		quote.State = state

		if state == cashu.PAID {
			quote.RequestPaid = true
		}

		c.JSON(200, quote)
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)

		if err != nil {
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		quote, err := database.GetMintQuoteById(pool, mintRequest.Quote)

		if err != nil {
			logger.Warn(fmt.Errorf("Incorrect body: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		if quote.Minted {
			log.Printf("Quote already minted")
			c.JSON(400, "Quote already minted")
			return
		}

		err = mint.ActiveQuotes.AddQuote(quote.Quote)

		if err != nil {
			logger.Warn(fmt.Errorf("AddActiveMintQuote: %w", err).Error())
			c.JSON(400, "Proof already being minted")
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
			// check all blind messages have the same unit
		}
		blindedSignatures := []cashu.BlindSignature{}
		recoverySigsDb := []cashu.RecoverSigDB{}
		lightningBackendType := ctx.Value("MINT_LIGHTNING_BACKEND").(string)

		switch lightningBackendType {

		case comms.FAKE_WALLET:
			invoice, err := zpay32.Decode(quote.Request, &mint.Network)

			if err != nil {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Error(fmt.Errorf("zpay32.Decode: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)

			if err != nil {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Warn(fmt.Errorf("UnmarshallAmt: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			// check the amount in outputs are the same as the quote
			if int32(*invoice.MilliSat) != int32(amountMilsats) {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Warn(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)).Error())
				c.JSON(403, "Amounts in outputs are not the same")
				return
			}
			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {

				mint.ActiveQuotes.RemoveQuote(quote.Quote)
				if errors.Is(err, m.ErrInvalidBlindMessage) {
					log.Printf("Invalid Blind Message %+v", m.ErrInvalidBlindMessage.Error())
					c.JSON(400, m.ErrInvalidBlindMessage.Error())
					return
				}

				c.JSON(500, "Opps!, something went wrong")
				return
			}

		case comms.LND_WALLET, comms.LNBITS_WALLET:

			state, _, err := mint.VerifyLightingPaymentHappened(ctx, pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMintPayStatus)
			if err != nil {
				mint.ActiveQuotes.RemoveQuote(quote.Quote)
				if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
					c.JSON(200, quote)
					return
				}
				logger.Warn(fmt.Errorf("VerifyLightingPaymentHappened: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			quote.State = state
			if quote.State == cashu.PAID {
				quote.RequestPaid = true
			} else {
				mint.ActiveQuotes.RemoveQuote(quote.Quote)
				log.Printf("Quote not paid")
				c.JSON(400, "Quote not paid")
				return
			}

			invoice, err := zpay32.Decode(quote.Request, &mint.Network)

			if err != nil {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)

			if err != nil {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Info(fmt.Errorf("UnmarshallAmt: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			// check the amount in outputs are the same as the quote
			if int32(*invoice.MilliSat) != int32(amountMilsats) {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Info(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)).Error())
				c.JSON(403, "Amounts in outputs are not the same")
				return
			}

			blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

			if err != nil {
				mint.RemoveActiveMintQuote(quote.Quote)
				logger.Error(fmt.Errorf("mint.SignBlindedMessages: %w", err).Error())
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
		quote.State = cashu.ISSUED

		err = database.ModifyQuoteMintMintedStatus(ctx, pool, quote.Minted, quote.State, quote.Quote)

		if err != nil {
			logger.Error(fmt.Errorf("ModifyQuoteMintMintedStatus: %w", err).Error())
			mint.RemoveActiveMintQuote(quote.Quote)
		}
		err = database.SetRestoreSigs(pool, recoverySigsDb)
		if err != nil {
			mint.RemoveActiveMintQuote(quote.Quote)
			logger.Error(fmt.Errorf("SetRecoverySigs: %w", err).Error())
			logger.Error(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb).Error())
			return
		}
		mint.ActiveQuotes.RemoveQuote(quote.Quote)

		// Store BlidedSignature
		c.JSON(200, cashu.PostMintBolt11Response{
			Signatures: blindedSignatures,
		})
	})
	v1.POST("/melt/quote/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltQuoteBolt11Request
		err := c.BindJSON(&meltRequest)

		if err != nil {
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		// TODO - REMOVE this when doing multi denomination tokens with Milisats
		if meltRequest.Unit != cashu.Sat.String() {
			log.Printf("Incorrect Unit for minting: %+v", meltRequest.Unit)
			c.JSON(400, "Incorrect Unit for melting")
			return
		}

		invoice, err := zpay32.Decode(meltRequest.Request, &mint.Network)
		if err != nil {
			logger.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		response := cashu.PostMeltQuoteBolt11Response{}
		dbRequest := cashu.MeltRequestDB{}

		expireTime := cashu.ExpiryTime()

		lightningBackendType := ctx.Value("MINT_LIGHTNING_BACKEND").(string)

		switch lightningBackendType {
		case comms.FAKE_WALLET:

			randUuid, err := uuid.NewRandom()

			if err != nil {
				logger.Info(fmt.Errorf("NewRamdom: %w", err).Error())

				c.JSON(500, "Opps!, something went wrong")
				return
			}

			response = cashu.PostMeltQuoteBolt11Response{
				Paid:            true,
				Expiry:          expireTime,
				FeeReserve:      1,
				Amount:          uint64(*invoice.MilliSat) / 1000,
				Quote:           randUuid.String(),
				State:           cashu.PAID,
				PaymentPreimage: "",
			}

			dbRequest = cashu.MeltRequestDB{
				Quote:           response.Quote,
				Request:         meltRequest.Request,
				Unit:            cashu.Sat.String(),
				Expiry:          response.Expiry,
				Amount:          response.Amount,
				FeeReserve:      response.FeeReserve,
				RequestPaid:     response.Paid,
				State:           response.State,
				PaymentPreimage: response.PaymentPreimage,
			}

		case comms.LND_WALLET, comms.LNBITS_WALLET:
			queryFee, err := mint.LightningComs.QueryPayment(invoice, meltRequest.Request)

			if err != nil {
				logger.Info(fmt.Errorf("mint.LightningComs.PayInvoice: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			hexHash := hex.EncodeToString(invoice.PaymentHash[:])

			response = cashu.PostMeltQuoteBolt11Response{
				Paid:            false,
				Expiry:          expireTime,
				FeeReserve:      (queryFee.FeeReserve + 1),
				Amount:          uint64(*invoice.MilliSat) / 1000,
				Quote:           hexHash,
				State:           cashu.UNPAID,
				PaymentPreimage: "",
			}

			dbRequest = cashu.MeltRequestDB{
				Quote:           response.Quote,
				Request:         meltRequest.Request,
				Unit:            cashu.Sat.String(),
				Expiry:          response.Expiry,
				Amount:          response.Amount,
				FeeReserve:      response.FeeReserve,
				RequestPaid:     response.Paid,
				State:           response.State,
				PaymentPreimage: response.PaymentPreimage,
			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		err = database.SaveQuoteMeltRequest(pool, dbRequest)

		if err != nil {
			logger.Warn(fmt.Errorf("SaveQuoteMeltRequest: %w", err).Error())
			logger.Warn(fmt.Errorf("dbRequest: %+v", dbRequest).Error())
			c.JSON(200, response)
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := database.GetMeltQuoteById(pool, quoteId)
		if err != nil {
			logger.Warn(fmt.Errorf("database.GetMeltQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			c.JSON(200, quote.GetPostMeltQuoteResponse())
			return
		}

		state, preimage, err := mint.VerifyLightingPaymentHappened(ctx, pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMeltPayStatus)
		if err != nil {
			if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
				c.JSON(200, quote.GetPostMeltQuoteResponse())
				return
			}
			logger.Warn(fmt.Errorf("VerifyLightingPaymentHappened: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		quote.PaymentPreimage = preimage
		quote.State = state
		if state == cashu.PAID {
			quote.RequestPaid = true
		}

		err = database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote)
		if err != nil {
			logger.Error(fmt.Errorf("database.GetMeltQuoteById: %w", err).Error())
			c.JSON(200, quote.GetPostMeltQuoteResponse())
			return
		}

		c.JSON(200, quote.GetPostMeltQuoteResponse())
	})

	v1.POST("/melt/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltBolt11Request
		err := c.BindJSON(&meltRequest)
		if err != nil {
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		if len(meltRequest.Inputs) == 0 {
			log.Printf("Outputs are empty")
			c.JSON(400, "Outputs are empty")
			return
		}

		quote, err := database.GetMeltQuoteById(pool, meltRequest.Quote)

		if err != nil {
			logger.Info(fmt.Errorf("GetMeltQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.Melted {
			log.Printf("Quote already melted")
			c.JSON(400, "Quote already melted")
			return
		}

		err = mint.AddQuotesAndProofs(quote.Quote, meltRequest.Inputs)

		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Warn(fmt.Errorf("mint.AddQuotesAndProofs(quote.Quote, meltRequest.Inputs): %w", err).Error())
			c.JSON(400, "Quote already being melted")
			return
		}

		unit, err := mint.CheckProofsAreSameUnit(meltRequest.Inputs)

		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Info(fmt.Sprintf("CheckProofsAreSameUnit: %+v", err))
			c.JSON(400, "Proofs are not the same unit")
			return
		}
		// TODO - REMOVE this when doing multi denomination tokens with Milisats
		if unit != cashu.Sat {
			log.Printf("Incorrect Unit for minting: %+v", unit)
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			c.JSON(400, "Incorrect Unit for minting")
			return
		}

		// check for needed amount of fees
		fee, err := cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()])
		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Info(fmt.Sprintf("cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()]): %+v", err))
			c.JSON(400, "Could not find keyset for proof id")
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
				mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
				logger.Info(fmt.Sprintf("proof.HashSecretToCurve(): %+v", err))
				c.JSON(400, "Problem processing proofs")
				return
			}

			meltRequest.Inputs[i] = p
			mint.PendingProofs = append(mint.PendingProofs, p)

		}

		if AmountProofs < (quote.Amount + quote.FeeReserve + uint64(fee)) {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Info(fmt.Sprintf("Not enought proofs to expend. Needs: %v", quote.Amount))
			c.JSON(403, "Not enought proofs to expend. Needs: %v")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := database.CheckListOfProofs(pool, CList, SecretList)

		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Warn(fmt.Sprintf("CheckListOfProofs: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		if len(knownProofs) != 0 {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			log.Printf("Proofs already used %+v \n", knownProofs)
			c.JSON(400, "Proofs already used")
			return
		}

		err = mint.VerifyListOfProofs(meltRequest.Inputs, []cashu.BlindedMessage{}, unit)

		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			log.Printf("Could not verify Proofs %+v", err.Error())

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
			case errors.Is(err, cashu.ErrLocktimePassed):
				c.JSON(403, cashu.ErrLocktimePassed.Error())
				return
			case errors.Is(err, cashu.ErrInvalidPreimage):
				c.JSON(403, cashu.ErrInvalidPreimage.Error())
				return

			}

			c.JSON(403, "Invalid Proof")
			return
		}

		lightningBackendType := ctx.Value("MINT_LIGHTNING_BACKEND").(string)

		var changeResponse []cashu.BlindSignature
		switch lightningBackendType {
		case comms.FAKE_WALLET:
			quote.RequestPaid = true
			quote.State = cashu.PAID
			quote.PaymentPreimage = "MockPaymentPreimage"

		case comms.LND_WALLET, comms.LNBITS_WALLET:
			payment, err := mint.LightningComs.PayInvoice(quote.Request, quote.FeeReserve)

			if err != nil {
				mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
				logger.Info(fmt.Sprintf("mint.LightningComs.PayInvoice %+v", err))
				c.JSON(400, "could not make payment")
				return
			}

			switch {
			case payment.PaymentError.Error() == "invoice is already paid":
				c.JSON(400, "invoice is already paid")
				return
			case payment.PaymentError.Error() == "unable to find a path to destination":
				c.JSON(400, "unable to find a path to destination")
				return
			case payment.PaymentError.Error() != "":
				logger.Info(fmt.Sprintf("unknown lighting error: %+v", payment.PaymentError))
				c.JSON(500, "Unknown error happend while paying")
				return

			}
			quote.RequestPaid = true
			quote.State = cashu.PAID
			quote.PaymentPreimage = payment.PaymentRequest

			// if fees where lower than expected return sats to the user
			feesInSat := uint64(1000 / 1000)

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
					mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
					logger.Warn(fmt.Errorf("mint.SignBlindedMessages: %w", err).Error())
					c.JSON(500, "Opps!, something went wrong")
					return
				}

				err = database.SetRestoreSigs(pool, recoverySigsDb)

				if err != nil {
					mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
					logger.Error(fmt.Errorf("SetRecoverySigs: %w", err).Error())
					logger.Error(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb).Error())
				}

				changeResponse = blindSignatures

			}

		default:
			log.Fatalf("Unknown lightning backend: %s", lightningBackendType)
		}

		quote.Melted = true

		response := quote.GetPostMeltQuoteResponse()
		response.Change = changeResponse

		err = database.ModifyQuoteMeltPayStatusAndMelted(pool, quote.RequestPaid, quote.Melted, quote.State, quote.Quote)
		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
			c.JSON(200, response)
			return
		}
		// send proofs to database
		err = database.SaveProofs(pool, meltRequest.Inputs)

		if err != nil {
			mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)
			logger.Error(fmt.Errorf("SaveProofs: %w", err).Error())
			logger.Error(fmt.Errorf("Proofs: %+v", meltRequest.Inputs).Error())
			c.JSON(200, response)
			return
		}
		mint.RemoveQuotesAndProofs(quote.Quote, meltRequest.Inputs)

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
