package routes

import (
	"context"
	"errors"
	"fmt"
	"log"
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

func v1bolt11Routes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		err := c.BindJSON(&mintRequest)

		if err != nil {
			slog.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		if mintRequest.Amount == 0 {
			slog.Info("Amount missing")
			c.JSON(400, "Amount missing")
			return
		}

		var mintRequestDB cashu.MintRequestDB
		if mint.Config.PEG_OUT_ONLY {
			slog.Info("Peg out only enables")
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.MINTING_DISABLED, nil))
			return
		}

		if mint.Config.PEG_IN_LIMIT_SATS != nil {
			if mintRequest.Amount > uint64(*mint.Config.PEG_IN_LIMIT_SATS) {
				slog.Info("Mint amount over the limit", slog.String(utils.LogExtraInfo, fmt.Sprint(mintRequest.Amount)))

				c.JSON(400, "Mint amount over the limit")
				return
			}

		}

		err = mint.VerifyUnitSupport(mintRequest.Unit)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyUnitSupport(mintRequest.Unit). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		slog.Debug(fmt.Sprintf("Requesting invoice for amount: %v. backend: %v", mintRequest.Amount, mint.LightningBackend.LightningType()))

		unit, err := cashu.UnitFromString(mintRequest.Unit)

		if err != nil {
			slog.Error(fmt.Errorf("cashu.UnitFromString(mintRequest.Unit). %w. %w", err, cashu.ErrUnitNotSupported).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		quoteId, err := utils.RandomHash()
		if err != nil {
			slog.Info("utils.RandomHash()", slog.String(utils.LogExtraInfo, fmt.Sprint(mintRequest.Amount)))
			c.JSON(500, "Opps! there was a problem with the mint")
			return
		}

		mintRequestDB = cashu.MintRequestDB{
			Quote:       quoteId,
			RequestPaid: false,
			Expiry:      expireTime,
			Unit:        unit.String(),
			State:       cashu.UNPAID,
			SeenAt:      now,
			Amount:      &mintRequest.Amount,
			Pubkey:      mintRequest.Pubkey,
		}

		resInvoice, err := mint.LightningBackend.RequestInvoice(mintRequestDB, cashu.Amount{Unit: unit, Amount: uint64(mintRequest.Amount)})
		if err != nil {
			slog.Info(err.Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if resInvoice.PaymentRequest == "" {
			slog.Error("The lightning backend is not returning an invoice.")
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		mintRequestDB.Request = resInvoice.PaymentRequest
		mintRequestDB.CheckingId = resInvoice.CheckingId

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		err = mint.MintDB.SaveMintRequest(tx, mintRequestDB)
		if err != nil {
			slog.Error(fmt.Errorf("SaveQuoteRequest: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		res := mintRequestDB.PostMintQuoteBolt11Response()
		c.JSON(200, res)
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
			slog.Error(fmt.Errorf("mint:quote mint.MintDB.GetMintRequestById(tx, quoteId): %w", err).Error())
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
			slog.Warn(fmt.Errorf("Mint decoding zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		quote, err = m.CheckMintRequest(mint, quote, invoice)
		if err != nil {
			slog.Warn(fmt.Errorf("m.CheckMintRequest(mint, quote): %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		err = mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)
		if err != nil {
			slog.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		res := quote.PostMintQuoteBolt11Response()
		c.JSON(200, res)
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)

		if err != nil {
			slog.Info(fmt.Sprintf("Incorrect body: %+v", err))
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

		mintRequestDB, err := mint.MintDB.GetMintRequestById(tx, mintRequest.Quote)

		if err != nil {
			slog.Error(fmt.Errorf(" mint-resquest mint.MintDB.GetMintRequestById(tx, mintRequest.Quote): %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		if mintRequestDB.Minted {
			slog.Warn("Quote already minted", slog.String(utils.LogExtraInfo, mintRequestDB.Quote))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TOKEN_ALREADY_ISSUED, nil))
			return
		}

		if mintRequestDB.Pubkey != nil {
			valid, err := mintRequest.VerifyPubkey(mintRequestDB.Pubkey)
			if err != nil {
				slog.Error(fmt.Errorf("Cold not verify signature: %w", err).Error())
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}

			if !valid {
				slog.Error(fmt.Errorf("Invalid signature: %w", err).Error())
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}
		}

		err = mint.VerifyUnitSupport(mintRequestDB.Unit)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyUnitSupport(quote.Unit). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		keysets, err := mint.Signer.GetKeysets()
		if err != nil {
			slog.Error(fmt.Errorf("mint.Signer.GetKeys(). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		_, err = mint.VerifyOutputs(mintRequest.Outputs, keysets.Keysets)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyOutputs(mintRequest.Outputs). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
			// check all blind messages have the same unit
		}
		invoice, err := zpay32.Decode(mintRequestDB.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			slog.Warn(fmt.Errorf("Mint decoding zpay32.Decode: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)
		if err != nil {
			slog.Info(fmt.Errorf("UnmarshallAmt: %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) < int32(amountMilsats) {
			slog.Info(fmt.Errorf("wrong amount of milisats: %v, needed %v", int32(*invoice.MilliSat), int32(amountMilsats)).Error())
			c.JSON(403, "Amounts in outputs are not the same")
			return
		}
		if !mintRequestDB.RequestPaid && mintRequestDB.State == cashu.UNPAID {

			mintRequestDB, err = m.CheckMintRequest(mint, mintRequestDB, invoice)
			if err != nil {
				if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
					c.JSON(200, mintRequestDB)
					return
				}
				slog.Warn(fmt.Errorf("m.CheckMintRequest(mint, quote): %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			err = mint.MintDB.ChangeMintRequestState(tx, mintRequestDB.Quote, mintRequestDB.RequestPaid, mintRequestDB.State, mintRequestDB.Minted)
			if err != nil {
				slog.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
				return
			}

			if mintRequestDB.State != cashu.PAID {
				c.JSON(400, cashu.ErrorCodeToResponse(cashu.REQUEST_NOT_PAID, nil))
				return
			}
		}

		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(mintRequest.Outputs)
		if err != nil {
			slog.Error(fmt.Errorf("mint.Signer.SignBlindMessages(mintRequest.Outputs): %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		mintRequestDB.Minted = true
		mintRequestDB.State = cashu.ISSUED

		err = mint.MintDB.ChangeMintRequestState(tx, mintRequestDB.Quote, mintRequestDB.RequestPaid, mintRequestDB.State, mintRequestDB.Minted)
		if err != nil {
			slog.Error(fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted): %w", err).Error())
			return
		}

		err = mint.MintDB.SaveRestoreSigs(tx, recoverySigsDb)
		if err != nil {
			slog.Error(fmt.Errorf("SetRecoverySigs on minting: %w", err).Error())
			slog.Error(fmt.Errorf("recoverySigsDb: %+v", recoverySigsDb).Error())
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		mint.Observer.SendMintEvent(mintRequestDB)
		// Store BlidedSignature
		c.JSON(200, cashu.PostMintBolt11Response{
			Signatures: blindedSignatures,
		})
	})

	v1.POST("/melt/quote/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltQuoteBolt11Request
		err := c.BindJSON(&meltRequest)

		if err != nil {
			slog.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		err = mint.VerifyUnitSupport(meltRequest.Unit)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyUnitSupport(quote.Unit). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		invoice, err := zpay32.Decode(meltRequest.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			slog.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
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
			slog.Info("utils.RandomHash()", slog.String(utils.LogExtraInfo, fmt.Sprint(meltRequest.Request)))
			c.JSON(500, "Opps! there was a problem with the mint")
			return
		}

		dbRequest := cashu.MeltRequestDB{}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		unit, err := cashu.UnitFromString(meltRequest.Unit)

		if err != nil {
			slog.Error(fmt.Errorf("cashu.UnitFromString(meltRequest.Unit). %w. %w", err, cashu.ErrUnitNotSupported).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		amount := invoice.MilliSat.ToSatoshis()
		cashuAmount := cashu.Amount{Unit: unit, Amount: uint64(amount)}

		isMpp := false
		mppAmount := cashu.Amount{Unit: cashu.Msat, Amount: uint64(meltRequest.IsMpp())}

		// if mpp is valid than change amount to mpp amount
		if mppAmount.Amount != 0 {
			isMpp = true
			if unit == cashu.Sat {
				err = mppAmount.To(cashu.Sat)
				if err != nil {
					slog.Error(fmt.Errorf("mppAmount.To(cashu.Sat). %w. %w", err, cashu.ErrUnitNotSupported).Error())
					errorCode, details := utils.ParseErrorToCashuErrorCode(err)
					c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
					return
				}

			}
			cashuAmount = mppAmount
		}

		if isMpp && !mint.LightningBackend.ActiveMPP() {
			slog.Info("Tried to do mpp when it is not available")
			c.JSON(400, "Sorry! MPP is not available")
			return
		}

		// check if it's internal transaction if it is and it's mpp error out

		isInternal, err := mint.IsInternalTransaction(meltRequest.Request)
		if err != nil {
			slog.Info(fmt.Errorf("mint.IsInternalTransaction(meltRequest.Request): %w", err).Error())
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if isMpp && isInternal {
			slog.Info(fmt.Sprint("Internal MPP not allowed", err))
			c.JSON(403, "Internal MPP not allowed")
			return
		}
		queryFee := uint64(0)
		checkingId := quoteId
		dbRequest = cashu.MeltRequestDB{
			Amount:          cashuAmount.Amount,
			Quote:           quoteId,
			Request:         meltRequest.Request,
			Unit:            unit.String(),
			Expiry:          expireTime,
			FeeReserve:      (queryFee + 1),
			RequestPaid:     false,
			State:           cashu.UNPAID,
			PaymentPreimage: "",
			SeenAt:          now,
			Mpp:             isMpp,
			CheckingId:      checkingId,
		}

		if !isInternal {
			feesResponse, err := mint.LightningBackend.QueryFees(meltRequest.Request, invoice, isMpp, cashuAmount)
			if err != nil {
				slog.Info(fmt.Errorf("mint.LightningBackend.QueryFees(meltRequest.Request, invoice, isMpp, cashuAmount): %w", err).Error())
				c.JSON(500, "Opps!, something went wrong")
				return
			}
			dbRequest.CheckingId = feesResponse.CheckingId
			dbRequest.FeeReserve = feesResponse.Fees.Amount
			dbRequest.Amount = feesResponse.AmountToSend.Amount
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			slog.Warn(fmt.Sprintf("m.MintDB.GetTx(ctx). %+v", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		log.Printf("dbRequest: %+v", dbRequest)
		err = mint.MintDB.SaveMeltRequest(tx, dbRequest)

		if err != nil {
			slog.Warn(fmt.Errorf("SaveQuoteMeltRequest: %w", err).Error())
			slog.Warn(fmt.Errorf("dbRequest: %+v", dbRequest).Error())
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			slog.Warn(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err).Error())
			return
		}

		c.JSON(200, dbRequest.GetPostMeltQuoteResponse())
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := mint.CheckMeltQuoteState(quoteId)
		if err != nil {
			slog.Error(fmt.Errorf("mint.CheckMeltQuoteState(quoteId). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote.GetPostMeltQuoteResponse())
	})

	v1.POST("/melt/bolt11", func(c *gin.Context) {
		log.Printf("\n\n melt Tryy")
		var meltRequest cashu.PostMeltBolt11Request
		err := c.BindJSON(&meltRequest)
		if err != nil {
			slog.Info(fmt.Sprintf("Incorrect body: %+v", err))
			c.JSON(400, "Malformed body request")
			return
		}

		quote, err := mint.Melt(meltRequest)
		if err != nil {
			slog.Error(fmt.Errorf("mint.Melt(meltRequest). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote)
	})
}
