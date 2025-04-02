package routes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
)

func v1bolt11Routes(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
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
			if mintRequest.Amount > uint64(*mint.Config.PEG_IN_LIMIT_SATS) {
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
		quoteId, err := utils.RandomHash()
		if err != nil {
			logger.Info("utils.RandomHash()", slog.String(utils.LogExtraInfo, fmt.Sprint(mintRequest.Amount)))
			c.JSON(500, "Opps! there was a problem with the mint")
			return
		}

		mintRequestDB = cashu.MintRequestDB{
			Quote:       quoteId,
			Request:     resInvoice.PaymentRequest,
			RequestPaid: false,
			Expiry:      expireTime,
			Unit:        mintRequest.Unit,
			State:       cashu.UNPAID,
			SeenAt:      now,
			Amount:      &mintRequest.Amount,
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		err = mint.MintDB.SaveMintRequest(tx, mintRequestDB)

		if err != nil {
			logger.Error(fmt.Errorf("SaveQuoteRequest: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		c.JSON(200, mintRequestDB.PostMintQuoteBolt11Response())
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		quote, err := mint.MintDB.GetMintRequestById(tx, quoteId)

		if err != nil {
			logger.Error(fmt.Errorf("mint:quote mint.MintDB.GetMintRequestById(tx, quoteId): %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return

		}

		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			c.JSON(200, quote)
			return
		}
		invoice, err := zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			logger.Warn(fmt.Errorf("Mint decoding zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		quote, err = m.CheckMintRequest(mint, quote, invoice)
		if err != nil {
			logger.Warn(fmt.Errorf("m.CheckMintRequest(mint, quote): %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		err = mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		c.JSON(200, quote.PostMintQuoteBolt11Response())
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)

		if err != nil {
			logger.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		quote, err := mint.MintDB.GetMintRequestById(tx, mintRequest.Quote)

		if err != nil {
			logger.Error(fmt.Errorf(" mint-resquest mint.MintDB.GetMintRequestById(tx, mintRequest.Quote): %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		if quote.Minted {
			logger.Warn("Quote already minted", slog.String(utils.LogExtraInfo, quote.Quote))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TOKEN_ALREADY_ISSUED, nil))
			return
		}

		keysets, err := mint.Signer.GetKeys()
		if err != nil {
			logger.Error(fmt.Errorf("mint.Signer.GetKeys(). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		_, err = mint.VerifyOutputs(mintRequest.Outputs, keysets.Keysets)
		if err != nil {
			logger.Error(fmt.Errorf("mint.VerifyOutputs(mintRequest.Outputs). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
			// check all blind messages have the same unit
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
		if !quote.RequestPaid && quote.State == cashu.UNPAID {

			quote, err = m.CheckMintRequest(mint, quote, invoice)
			if err != nil {
				if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
					c.JSON(200, quote)
					return
				}
				logger.Warn(fmt.Errorf("m.CheckMintRequest(mint, quote): %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			err = mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)
			if err != nil {
				logger.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
				return
			}

			if quote.State != cashu.PAID {
				c.JSON(400, cashu.ErrorCodeToResponse(cashu.REQUEST_NOT_PAID, nil))
				return
			}
		}

		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(mintRequest.Outputs)
		if err != nil {
			logger.Error(fmt.Errorf("mint.Signer.SignBlindMessages(mintRequest.Outputs): %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		quote.Minted = true
		quote.State = cashu.ISSUED

		err = mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
			return
		}

		err = mint.MintDB.SaveRestoreSigs(tx, recoverySigsDb)
		if err != nil {
			logger.Error(fmt.Errorf("SetRecoverySigs on minting: %w", err).Error())
			logger.Error(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb).Error())
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		mint.Observer.SendMintEvent(quote)
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

		quoteId, err := utils.RandomHash()
		if err != nil {
			logger.Info("utils.RandomHash()", slog.String(utils.LogExtraInfo, fmt.Sprint(meltRequest.Request)))
			c.JSON(500, "Opps! there was a problem with the mint")
			return
		}

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

		// check if it's internal transaction if it is and it's mpp error out

		isInternal, err := mint.IsInternalTransaction(meltRequest.Request)
		if err != nil {
			logger.Info(fmt.Errorf("mint.IsInternalTransaction(meltRequest.Request): %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if isMpp && isInternal {
			logger.Info(fmt.Sprint("Internal MPP not allowed", err))
			c.JSON(403, "Internal MPP not allowed")
			return
		}
		queryFee := uint64(0)
		if !isInternal {
			queryFee, err = mint.LightningBackend.QueryFees(meltRequest.Request, invoice, isMpp, amount)
			if err != nil {
				logger.Info(fmt.Errorf("mint.LightningComs.PayInvoice: %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

		}

		dbRequest = cashu.MeltRequestDB{
			Amount:          amount,
			Quote:           quoteId,
			Request:         meltRequest.Request,
			Unit:            meltRequest.Unit,
			Expiry:          expireTime,
			FeeReserve:      (queryFee + 1),
			RequestPaid:     false,
			State:           cashu.UNPAID,
			PaymentPreimage: "",
			SeenAt:          now,
			Mpp:             isMpp,
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			logger.Warn(fmt.Sprintf("m.MintDB.GetTx(ctx). %+v", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		err = mint.MintDB.SaveMeltRequest(tx, dbRequest)

		if err != nil {
			logger.Warn(fmt.Errorf("SaveQuoteMeltRequest: %w", err).Error())
			logger.Warn(fmt.Errorf("dbRequest: %+v", dbRequest).Error())
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			logger.Warn(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err).Error())
			return
		}

		c.JSON(200, dbRequest.GetPostMeltQuoteResponse())
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := mint.CheckMeltQuoteState(quoteId)
		if err != nil {
			logger.Error(fmt.Errorf("mint.Melt(meltRequest, logger ). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
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

		quote, err := mint.Melt(meltRequest, logger)
		if err != nil {
			logger.Error(fmt.Errorf("mint.Melt(meltRequest, logger ). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote)
	})
}
