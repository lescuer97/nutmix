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

func v1bolt11Routes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		err := c.BindJSON(&mintRequest)

		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
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
				slog.Info("Mint amount over the limit", slog.Uint64("amount", mintRequest.Amount))

				c.JSON(400, "Mint amount over the limit")
				return
			}

		}

		err = mint.VerifyUnitSupport(mintRequest.Unit)
		if err != nil {
			slog.Error("mint.VerifyUnitSupport(mintRequest.Unit)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		slog.Debug("Requesting invoice for amount",
			slog.Uint64("amount", mintRequest.Amount),
			slog.Any("backend", mint.LightningBackend.LightningType()))

		unit, err := cashu.UnitFromString(mintRequest.Unit)

		if err != nil {
			slog.Error("cashu.UnitFromString(mintRequest.Unit)", slog.Any("error", err), slog.Any("err", cashu.ErrUnitNotSupported))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		quoteId, err := utils.RandomHash()
		if err != nil {
			slog.Info("utils.RandomHash()", slog.Uint64("amount", mintRequest.Amount))
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
			slog.Info("Payment request", slog.Any("error", err))
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
			slog.Error("SaveQuoteRequest", slog.Any("error", err))
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
			slog.Error("mint:quote mint.MintDB.GetMintRequestById(tx, quoteId)", slog.Any("error", err))
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
			slog.Warn("Mint decoding zpay32.Decode", slog.Any("error", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		quote, err = m.CheckMintRequest(mint, quote, invoice)
		if err != nil {
			slog.Warn("m.CheckMintRequest(mint, quote)", slog.Any("error", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		err = mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)
		if err != nil {
			slog.Error("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)", slog.Any("error", err))
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
			slog.Info("Incorrect body", slog.Any("error", err))
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
			slog.Error(" mint-resquest mint.MintDB.GetMintRequestById(tx, mintRequest.Quote)", slog.Any("error", err))
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
				slog.Error("Cold not verify signature", slog.Any("error", err))
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}

			if !valid {
				slog.Error("Invalid signature", slog.Any("error", err))
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}
		}

		err = mint.VerifyUnitSupport(mintRequestDB.Unit)
		if err != nil {
			slog.Error("mint.VerifyUnitSupport(quote.Unit)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		keysets, err := mint.Signer.GetKeysets()
		if err != nil {
			slog.Error("mint.Signer.GetKeys()", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		_, err = mint.VerifyOutputs(mintRequest.Outputs, keysets.Keysets)
		if err != nil {
			slog.Error("mint.VerifyOutputs(mintRequest.Outputs)", slog.Any("error", err))
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
			slog.Warn("Mint decoding zpay32.Decode", slog.Any("error", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		amountMilsats, err := lnrpc.UnmarshallAmt(int64(amountBlindMessages), 0)
		if err != nil {
			slog.Info("UnmarshallAmt", slog.Any("error", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) < int32(amountMilsats) {
			slog.Info("wrong amount of milisats", slog.Int("invoice_milisats", int(*invoice.MilliSat)), slog.Int("needed_milisats", int(amountMilsats)))
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
				slog.Warn("m.CheckMintRequest(mint, quote)", slog.Any("error", err))
				c.JSON(500, "Opps!, something went wrong")
				return
			}

			err = mint.MintDB.ChangeMintRequestState(tx, mintRequestDB.Quote, mintRequestDB.RequestPaid, mintRequestDB.State, mintRequestDB.Minted)
			if err != nil {
				slog.Error("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)", slog.Any("error", err))
				return
			}

			if mintRequestDB.State != cashu.PAID {
				c.JSON(400, cashu.ErrorCodeToResponse(cashu.REQUEST_NOT_PAID, nil))
				return
			}
		}

		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(mintRequest.Outputs)
		if err != nil {
			slog.Error("mint.Signer.SignBlindMessages(mintRequest.Outputs)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		mintRequestDB.Minted = true
		mintRequestDB.State = cashu.ISSUED

		err = mint.MintDB.ChangeMintRequestState(tx, mintRequestDB.Quote, mintRequestDB.RequestPaid, mintRequestDB.State, mintRequestDB.Minted)
		if err != nil {
			slog.Error("mint.MintDB.ChangeMintRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Minted)", slog.Any("error", err))
			return
		}

		err = mint.MintDB.SaveRestoreSigs(tx, recoverySigsDb)
		if err != nil {
			slog.Error("SetRecoverySigs on minting", slog.Any("error", err))
			slog.Error("recoverySigsDb", slog.Any("recovery_sigs", recoverySigsDb))
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
			slog.Info("Incorrect body", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		err = mint.VerifyUnitSupport(meltRequest.Unit)
		if err != nil {
			slog.Error("mint.VerifyUnitSupport(quote.Unit)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		invoice, err := zpay32.Decode(meltRequest.Request, mint.LightningBackend.GetNetwork())
		if err != nil {
			slog.Info("zpay32.Decode", slog.Any("error", err))
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
			slog.Info("utils.RandomHash()", slog.String(utils.LogExtraInfo, meltRequest.Request))
			c.JSON(500, "Opps! there was a problem with the mint")
			return
		}

		dbRequest := cashu.MeltRequestDB{}

		expireTime := cashu.ExpiryTimeMinUnit(15)
		now := time.Now().Unix()

		unit, err := cashu.UnitFromString(meltRequest.Unit)

		if err != nil {
			slog.Error("cashu.UnitFromString(meltRequest.Unit)", slog.Any("error", err), slog.Any("err", cashu.ErrUnitNotSupported))
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
					slog.Error("mppAmount.To(cashu.Sat)", slog.Any("error", err), slog.Any("err", cashu.ErrUnitNotSupported))
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
			slog.Info("mint.IsInternalTransaction(meltRequest.Request)", slog.Any("error", err))
			c.JSON(500, "Opps!, something went wrong")
			return
		}

		if isMpp && isInternal {
			slog.Info("Internal MPP not allowed", slog.Any("error", err))
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
				slog.Info("mint.LightningBackend.QueryFees(meltRequest.Request, invoice, isMpp, cashuAmount)", slog.Any("error", err))
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
			slog.Warn("m.MintDB.GetTx(ctx)", slog.Any("error", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		err = mint.MintDB.SaveMeltRequest(tx, dbRequest)

		if err != nil {
			slog.Warn("SaveQuoteMeltRequest", slog.Any("error", err))
			slog.Warn("dbRequest", slog.Any("db_request", dbRequest))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			slog.Warn("mint.MintDB.Commit(ctx tx)", slog.Any("error", err))
			return
		}

		c.JSON(200, dbRequest.GetPostMeltQuoteResponse())
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := mint.CheckMeltQuoteState(quoteId)
		if err != nil {
			slog.Error("mint.CheckMeltQuoteState(quoteId)", slog.Any("error", err))
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
			slog.Info("Incorrect body", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		quote, err := mint.Melt(meltRequest)
		if err != nil {
			slog.Error("mint.Melt(meltRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote)
	})
}
