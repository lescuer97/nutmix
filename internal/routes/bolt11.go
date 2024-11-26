package routes

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"log/slog"
	"slices"
	"strings"
	"time"
)

func v1bolt11Routes(r *gin.Engine, mint *mint.Mint, logger *slog.Logger) {
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
			logger.Warn("Incorrect Unit for minting: %+v", slog.String(utils.LogExtraInfo, mintRequest.Unit))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNIT_NOT_SUPPORTED, nil))
			return
		}

		if mintRequest.Amount == 0 {
			logger.Info("Amount missing")
			c.JSON(400, "Amount missing")
			return
		}

		var mintRequestDB cashu.MintRequestDB
		if mint.Config.PEG_OUT_ONLY {
			logger.Info("Peg out only enables")
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.MINTING_DISABLED, nil))
			return
		}

		if mint.Config.PEG_IN_LIMIT_SATS != nil {
			if mintRequest.Amount > int64(*mint.Config.PEG_IN_LIMIT_SATS) {
				logger.Info("Mint amount over the limit", slog.String(utils.LogExtraInfo, fmt.Sprint(mintRequest.Amount)))

				c.JSON(400, "Mint amount over the limit")
				return
			}

		}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		logger.Debug(fmt.Sprintf("Requesting invoice for amount: %v. backend: %v", mintRequest.Amount, mint.LightningBackend.LightningType()))

		resInvoice, err := mint.LightningBackend.RequestInvoice(mintRequest.Amount)

		if err != nil {
			logger.Info(err.Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		mintRequestDB = cashu.MintRequestDB{
			Quote:       resInvoice.Rhash,
			Request:     resInvoice.PaymentRequest,
			RequestPaid: false,
			Expiry:      expireTime,
			Unit:        mintRequest.Unit,
			State:       cashu.UNPAID,
			SeenAt:      now,
		}

		if mint.LightningBackend.LightningType() == lightning.FAKEWALLET {
			mintRequestDB.RequestPaid = true
			mintRequestDB.State = cashu.PAID
		}

		err = mint.MintDB.SaveMintRequest(mintRequestDB)

		if err != nil {
			logger.Error(fmt.Errorf("SaveQuoteRequest: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		c.JSON(200, mintRequestDB.PostMintQuoteBolt11Response())
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := mint.MintDB.GetMintRequestById(quoteId)

		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			c.JSON(200, quote)
			return
		}
		if err != nil {
			logger.Error(fmt.Errorf("GetMintQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		state, _, err := mint.VerifyLightingPaymentHappened(quote.RequestPaid, quote.Quote, mint.MintDB.ChangeMintRequestState)

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
		err = mint.ActiveQuotes.AddQuote(mintRequest.Quote)

		if err != nil {
			logger.Warn(fmt.Errorf("AddActiveMintQuote: %w", err).Error())
			c.JSON(400, "Proof already being minted")
			return
		}

		defer mint.ActiveQuotes.RemoveQuote(mintRequest.Quote)
		quote, err := mint.MintDB.GetMintRequestById(mintRequest.Quote)

		if err != nil {
			logger.Error(fmt.Errorf("Incorrect body: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.Minted {
			logger.Warn("Quote already minted", slog.String(utils.LogExtraInfo, quote.Quote))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TOKEN_ALREADY_ISSUED, nil))
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
			// check all blind messages have the same unit
		}
		blindedSignatures := []cashu.BlindSignature{}
		recoverySigsDb := []cashu.RecoverSigDB{}

		state, _, err := mint.VerifyLightingPaymentHappened(quote.RequestPaid, quote.Quote, mint.MintDB.ChangeMintRequestState)

		if err != nil {
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
			logger.Debug("Quote not paid")
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.REQUEST_NOT_PAID, nil))
			return
		}

		invoice, err := zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork())

		if err != nil {
			logger.Warn(fmt.Errorf("Mint decoding zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)

		if err != nil {
			logger.Info(fmt.Errorf("UnmarshallAmt: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) != int32(amountMilsats) {
			logger.Info(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)).Error())
			c.JSON(403, "Amounts in outputs are not the same")
			return
		}

		blindedSignatures, recoverySigsDb, err = mint.SignBlindedMessages(mintRequest.Outputs, quote.Unit)

		if err != nil {
			logger.Error(fmt.Errorf("mint.SignBlindedMessages: %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		quote.Minted = true
		quote.State = cashu.ISSUED

		err = mint.MintDB.ChangeMintRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Minted)

		if err != nil {
			logger.Error(fmt.Errorf("ModifyQuoteMintMintedStatus: %w", err).Error())
		}
		err = mint.MintDB.SaveRestoreSigs(recoverySigsDb)
		if err != nil {
			logger.Error(fmt.Errorf("SetRecoverySigs: %w", err).Error())
			logger.Error(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb).Error())
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
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		// TODO - REMOVE this when doing multi denomination tokens with Milisats
		if meltRequest.Unit != cashu.Sat.String() {
			logger.Info("Incorrect Unit for minting", slog.String(utils.LogExtraInfo, meltRequest.Unit))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNIT_NOT_SUPPORTED, nil))
			return
		}

		invoice, err := zpay32.Decode(meltRequest.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			logger.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if uint64(*invoice.MilliSat) == 0 {
			c.JSON(400, "Invoice has no amount")
			return
		}

		if mint.Config.PEG_OUT_LIMIT_SATS != nil {
			if int64(*invoice.MilliSat) > (int64(*mint.Config.PEG_OUT_LIMIT_SATS) * 1000) {
				c.JSON(400, "Melt amount over the limit")
				return
			}

		}

		response := cashu.PostMeltQuoteBolt11Response{}
		dbRequest := cashu.MeltRequestDB{}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		amount := uint64(*invoice.MilliSat) / 1000

		isMpp := false
		mppAmount := meltRequest.IsMpp()

		// if mpp is valid than change amount to mpp amount
		if mppAmount != 0 {
			isMpp = true
			amount = mppAmount
		}

		if isMpp && !mint.LightningBackend.ActiveMPP() {
			logger.Info("Tried to do mpp when it is not available")
			c.JSON(400, "Sorry! MPP is not available")
			return
		}
		queryFee, err := mint.LightningBackend.QueryFees(meltRequest.Request, invoice, isMpp, amount)

		if err != nil {
			logger.Info(fmt.Errorf("mint.LightningComs.PayInvoice: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		hexHash := hex.EncodeToString(invoice.PaymentHash[:])

		response = cashu.PostMeltQuoteBolt11Response{
			Paid:            false,
			Expiry:          expireTime,
			FeeReserve:      (queryFee + 1),
			Amount:          amount,
			Quote:           hexHash,
			State:           cashu.UNPAID,
			PaymentPreimage: "",
		}
		if mint.LightningBackend.LightningType() == lightning.FAKEWALLET {
			response.Paid = true
			response.State = cashu.PAID
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
			SeenAt:          now,
			Mpp:             isMpp,
		}

		err = mint.MintDB.SaveMeltRequest(dbRequest)

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

		quote, err := mint.MintDB.GetMeltRequestById(quoteId)
		if err != nil {
			logger.Warn(fmt.Errorf("database.GetMeltQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			c.JSON(200, quote.GetPostMeltQuoteResponse())
			return
		}

		state, preimage, err := mint.VerifyLightingPaymentHappened(quote.RequestPaid, quote.Quote, mint.MintDB.ChangeMeltRequestState)
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

		err = mint.MintDB.AddPreimageMeltRequest(quote.Quote, preimage)
		if err != nil {
			logger.Error(fmt.Errorf("database.AddPaymentPreimageToMeltRequest(pool, : %w", err).Error())
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
			logger.Info("Outputs are empty")
			c.JSON(400, "Outputs are empty")
			return
		}
		err = mint.AddQuotesAndProofs(meltRequest.Quote, meltRequest.Inputs)

		if err != nil {
			logger.Warn(fmt.Errorf("mint.AddQuotesAndProofs(quote.Quote, meltRequest.Inputs): %w", err).Error())
			c.JSON(400, "Quote already being melted")
			return
		}

		defer mint.RemoveQuotesAndProofs(meltRequest.Quote, meltRequest.Inputs)
		quote, err := mint.MintDB.GetMeltRequestById(meltRequest.Quote)

		if err != nil {
			logger.Info(fmt.Errorf("GetMeltQuoteById: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if quote.State == cashu.PENDING {
			logger.Warn("Quote is pending")
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.QUOTE_PENDING, nil))
			return
		}

		if quote.Melted {
			logger.Info("Quote already melted", slog.String(utils.LogExtraInfo, quote.Quote))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.INVOICE_ALREADY_PAID, nil))
			return
		}

		unit, err := mint.CheckProofsAreSameUnit(meltRequest.Inputs)

		if err != nil {
			logger.Info(fmt.Sprintf("CheckProofsAreSameUnit: %+v", err))
			c.JSON(400, "Proofs are not the same unit")
			return
		}

		// TODO - REMOVE this when doing multi denomination tokens with Milisats
		if unit != cashu.Sat {
			logger.Info("Incorrect Unit for minting", slog.String(utils.LogExtraInfo, quote.Unit))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNIT_NOT_SUPPORTED, nil))
			return
		}

		// check for needed amount of fees
		fee, err := cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()])
		if err != nil {
			logger.Info(fmt.Sprintf("cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()]): %+v", err))
			c.JSON(400, "Could not find keyset for proof id")
			return
		}

		AmountProofs, SecretsList, err := utils.GetAndCalculateProofsValues(&meltRequest.Inputs)
		if err != nil {
			logger.Warn("utils.GetProofsValues(&meltRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, "Problem processing proofs")
			return
		}

		// change state to pending
		meltRequest.Inputs.SetPendingAndQuoteRef(quote.Quote)
		quote.State = cashu.PENDING

		if AmountProofs < (quote.Amount + quote.FeeReserve + uint64(fee)) {
			logger.Info(fmt.Sprintf("Not enought proofs to expend. Needs: %v", quote.Amount))
			c.JSON(403, "Not enought proofs to expend. Needs: %v")
			return
		}

		// check if we know any of the proofs
		knownProofs, err := mint.MintDB.GetProofsFromSecret(SecretsList)

		if err != nil {
			logger.Warn(fmt.Sprintf("CheckListOfProofs: %+v", err))
			c.JSON(500, "Opps! there was an issue")
			return
		}

		if len(knownProofs) != 0 {
            logger.Info("Proofs already used", slog.String(utils.LogExtraInfo, fmt.Sprintf("knownproofs:  %+v", knownProofs)))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TOKEN_ALREADY_SPENT, nil))
			return
		}

		err = mint.VerifyListOfProofs(meltRequest.Inputs, []cashu.BlindedMessage{}, unit)

		if err != nil {
			logger.Debug("Could not verify Proofs", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		invoice, err := zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			logger.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		var paidLightningFeeSat uint64

		payment, err := mint.LightningBackend.PayInvoice(quote.Request, invoice, quote.FeeReserve, quote.Mpp, quote.Amount)

		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN {
            logger.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, err := mint.LightningBackend.CheckPayed(quote.Quote)

			// if error on checking payement we will save as pending and returns status
			if err != nil {

				response := quote.GetPostMeltQuoteResponse()
				err = mint.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted)
				if err != nil {
					logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
				}

				// Save proofs with pending state
				err = mint.MintDB.SaveProof(meltRequest.Inputs)
				if err != nil {
					logger.Error(fmt.Errorf("SaveProofs: %w", err).Error())
					logger.Error(fmt.Errorf("Proofs: %+v", meltRequest.Inputs).Error())
					c.JSON(200, response)
					return
				}

				c.JSON(200, response)
				return
			}

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				quote.State = cashu.PENDING

				response := quote.GetPostMeltQuoteResponse()
				// change melt request state
				err = mint.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted)
				if err != nil {
					logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
				}

				// Save proofs with pending state
				err = mint.MintDB.SaveProof(meltRequest.Inputs)
				if err != nil {
					logger.Error(fmt.Errorf("SaveProofs: %w", err).Error())
					logger.Error(fmt.Errorf("Proofs: %+v", meltRequest.Inputs).Error())
					c.JSON(200, response)
					return
				}

				c.JSON(200, response)
				return

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				logger.Info(fmt.Sprintf("mint.LightningComs.PayInvoice %+v", err))
				c.JSON(400, "could not make payment")
				return
			}
		}

		if payment.PaymentState == lightning.SETTLED {
			quote.RequestPaid = true
			quote.State = cashu.PAID
			quote.PaymentPreimage = payment.Preimage
		}

		quote.Melted = true
		response := quote.GetPostMeltQuoteResponse()

		// if fees where lower than expected return sats to the user
		paidLightningFeeSat = uint64(payment.PaidFeeSat)

		//  if total expent is lower that the amount of proofs that where given
		//  change is returned
		totalExpent := quote.Amount + paidLightningFeeSat + uint64(fee)
		if AmountProofs > totalExpent && len(meltRequest.Outputs) > 0 {

			overpaidFees := AmountProofs - totalExpent
			change := utils.GetChangeOutput(overpaidFees, meltRequest.Outputs)

			blindSignatures, recoverySigsDb, err := mint.SignBlindedMessages(change, quote.Unit)

			if err != nil {
				logger.Info("mint.SignBlindedMessages", slog.String(utils.LogExtraInfo, err.Error()))
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}

			err = mint.MintDB.SaveRestoreSigs(recoverySigsDb)

			if err != nil {
				logger.Error("database.SetRestoreSigs", slog.String(utils.LogExtraInfo, err.Error()))
				logger.Error("recoverySigsDb", slog.String(utils.LogExtraInfo, fmt.Sprintf("%+v", recoverySigsDb)))
			}

			response.Change = blindSignatures
		}

		err = mint.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted)
		if err != nil {
			logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
			c.JSON(200, response)
			return
		}

		err = mint.MintDB.AddPreimageMeltRequest(quote.Quote, quote.PaymentPreimage)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.AddPreimageMeltRequest(quote.Quote, quote.PaymentPreimage) %+v", err).Error())
			c.JSON(200, response)
			return
		}

		// change proofs to spent
		meltRequest.Inputs.SetProofsState(cashu.PROOF_SPENT)

		// send proofs to database
		err = mint.MintDB.SaveProof(meltRequest.Inputs)

		if err != nil {
			logger.Error(fmt.Errorf("SaveProofs: %w", err).Error())
			logger.Error(fmt.Errorf("Proofs: %+v", meltRequest.Inputs).Error())
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
