package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
	qrcode "github.com/skip2/go-qrcode"
)

func LiquidityButton(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		if !utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND) {
			// c.Status(200)
			return
		}
		component := templates.LiquidityButton()

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}

		return
	}
}

// LnSendPage handles the send funds page
func LnSendPage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		var balance string
		if err != nil {
			slog.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))
			balance = "Unavailable"
		} else {
			balance = strconv.FormatUint(milillisatBalance/1000, 10)
		}

		component := templates.LnSendPage(balance)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

// LnReceivePage handles the receive funds page
func LnReceivePage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		component := templates.LnReceivePage()

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

// swaps out of the mint
func SwapOutForm(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {

			slog.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			c.Error(fmt.Errorf("mint.LightningComs.WalletBalance(). %w", err))
			return
		}

		balance := strconv.FormatUint(milillisatBalance/1000, 10)
		component := templates.SwapOutPostForm(balance)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}

		return
	}
}

// Swaps into the mint
func LightningSwapForm() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		component := templates.SwapInPostForm()

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}

func SwapOutRequest(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// need amount and liquid address
		invoice := c.PostForm("invoice")

		decodedInvoice, err := zpay32.Decode(invoice, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			slog.Warn("zpay32.Decode(invoice)", slog.Any("error", err))
			if err := RenderError(c, "Invalid Lightning Invoice"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		if decodedInvoice.MilliSat == nil {
			if err := RenderError(c, "Invoice must have an amount"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		// Check for sufficient funds
		currentBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {
			slog.Error("Could not fetch wallet balance", slog.Any("error", err))
			if err := RenderError(c, "Could not check wallet balance"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		invoiceAmountMsats := uint64(*decodedInvoice.MilliSat)
		if currentBalance < invoiceAmountMsats {
			if err := RenderError(c, fmt.Sprintf("Insufficient funds: Have %d sats, need %d sats", currentBalance/1000, invoiceAmountMsats/1000)); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		amount := decodedInvoice.MilliSat.ToSatoshis()
		feesResponse, err := mint.LightningBackend.QueryFees(invoice, decodedInvoice, false, cashu.Amount{Unit: cashu.Sat, Amount: uint64(amount)})
		if err != nil {
			slog.Info("mint.LightningComs.PayInvoice", slog.Any("error", err))
			if err := RenderError(c, "Could not calculate fees or route not found"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		uuid := uuid.New().String()
		swap := utils.LiquiditySwap{
			Amount:           uint64(amount),
			LightningInvoice: invoice,
			State:            utils.WaitingUserConfirmation,
			Id:               uuid,
			Type:             utils.LiquidityOut,
			CheckingId:       feesResponse.CheckingId,
		}

		now := decodedInvoice.Timestamp.Add(decodedInvoice.Expiry()).Unix()
		swap.Expiration = uint64(now)

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			slog.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			}
		}()

		err = mint.MintDB.AddLiquiditySwap(tx, swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			c.Error(fmt.Errorf("\n Could not add swap request %+v \n", err))
			return
		}
		err = mint.MintDB.Commit(context.Background(), tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(context.Background(), tx). %w", err))
			return
		}

		c.Header("HX-Location", "/admin/liquidity/"+uuid)
		component := templates.LightningSendSummary(decodedInvoice.MilliSat.ToSatoshis().Format(btcutil.AmountSatoshi), swap.LightningInvoice, uuid)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New(`templates.LiquidSwapSummary(decodedInvoice.MilliSat.ToSatoshis().String(), string(amount),  "test address", uuid)`))
			return
		}

		return
	}
}

func SwapInRequest(mint *m.Mint, newLiquidity chan string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// only needs the amount and we generate an invoice from the mint directly
		amountStr := c.PostForm("amount")

		amount, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil {
			if err := RenderError(c, "Invalid amount"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		if amount <= 0 {
			if err := RenderError(c, "Amount must be greater than 0"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}

		uuid := uuid.New().String()

		resp, err := mint.LightningBackend.RequestInvoice(cashu.MintRequestDB{Quote: uuid}, cashu.Amount{Amount: amount, Unit: cashu.Sat})
		if err != nil {
			slog.Error("mint.LightningBackend.RequestInvoice", slog.Any("error", err))
			if err := RenderError(c, "Could not generate invoice"); err != nil {
				slog.Error("failed to render error", slog.Any("error", err))
			}
			return
		}
		swap := utils.LiquiditySwap{
			Amount:           amount,
			LightningInvoice: resp.PaymentRequest,
			State:            utils.MintWaitingPaymentRecv,
			Id:               uuid,
			Type:             utils.LiquidityIn,
			CheckingId:       resp.CheckingId,
		}

		decodedInvoice, err := zpay32.Decode(resp.PaymentRequest, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n zpay32.Decode(resp.PaymentRequest, %+v \n", err)
			c.Error(fmt.Errorf("\n zpay32.Decode(resp.PaymentRequest, %+v \n", err))
			return
		}

		now := decodedInvoice.Timestamp.Add(decodedInvoice.Expiry()).Unix()
		swap.Expiration = uint64(now)

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			slog.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			}
		}()

		err = mint.MintDB.AddLiquiditySwap(tx, swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			c.Error(fmt.Errorf("\n Could not add swap request %+v \n", err))
			return
		}
		err = mint.MintDB.Commit(context.Background(), tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(context.Background(), tx). %w", err))
			return
		}

		amountConverted := strconv.FormatUint(swap.Amount, 10)

		// generate qrCode
		qrcode, err := generateQR(swap.LightningInvoice)
		if err != nil {
			c.Error(fmt.Errorf("generateQR(swap.LightningInvoice). %w", err))
			return
		}
		newLiquidity <- uuid

		// redirect to the swap status page
		c.Header("HX-Location", "/admin/liquidity/"+uuid)
		component := templates.LightningReceiveSummary(amountConverted, swap.LightningInvoice, qrcode, swap.Id)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}

		return
	}
}

func SwapStateCheck(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		// only needs the amount and we generate an invoice from the mint directly
		swapId := c.Param("swapId")

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			slog.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			}
		}()

		swapRequest, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.GetLiquiditySwapById(swapId). %w", err))
			return
		}
		if err := tx.Commit(context.Background()); err != nil {
			_ = c.Error(fmt.Errorf("tx.Commit failed: %w", err))
			return
		}

		component := templates.SwapState(swapRequest.State, swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}
		return

	}
}

func ConfirmSwapOutTransaction(mint *m.Mint, newLiquidity chan string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// only needs the amount and we generate an invoice from the mint directly
		swapId := c.Param("swapId")
		if swapId == "" {
			c.Error(fmt.Errorf("swapId is required"))
			return
		}

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			slog.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			}
		}()

		swapRequest, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			c.Error(errors.New("mint.MintDB.GetLiquiditySwapById(swapId)"))
			return
		}

		if swapRequest.State != utils.WaitingUserConfirmation {
			c.Error(fmt.Errorf("Can't pay lightning invoice %w", utils.ErrAlreadyLNPaying))
			return
		}

		swapRequest.State = utils.LightningPaymentPending
		err = mint.MintDB.ChangeLiquiditySwapState(tx, swapId, swapRequest.State)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapId, swapRequest.State). %w", err))
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			slog.Error("Failed to commit transaction", slog.Any("error", err))
			return
		}
		newLiquidity <- swapId

		decodedInvoice, err := zpay32.Decode(swapRequest.LightningInvoice, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			c.Error(fmt.Errorf("zpay32.Decode(res.Destination) %w", err))
			return
		}

		fee := uint64(float64(swapRequest.Amount) * 0.10)

		slog.Info("making payment to invoice", slog.String("invoice", swapRequest.LightningInvoice))

		payment, err := mint.LightningBackend.PayInvoice(cashu.MeltRequestDB{Request: swapRequest.LightningInvoice}, decodedInvoice, fee, false, cashu.Amount{Unit: cashu.Sat, Amount: swapRequest.Amount})

		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN || payment.PaymentState == lightning.PENDING {

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, _, err := mint.LightningBackend.CheckPayed(swapRequest.LightningInvoice, decodedInvoice, swapRequest.CheckingId)

			// if error on checking payement we will save as pending and returns status
			if err != nil {
				slog.Warn("Something happened while paying the invoice. Keeping proofs and quote as pending ")
				return
			}

			slog.Info("after check payed verification")
			lnStatusTx, err := mint.MintDB.GetTx(ctx)
			if err != nil {
				return
			}
			defer func() {
				if err := lnStatusTx.Rollback(ctx); err != nil {
					slog.Warn("rollback error", slog.Any("error", err))
				}
			}()

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				err = mint.MintDB.ChangeLiquiditySwapState(lnStatusTx, swapRequest.Id, utils.LightningPaymentPending)
				if err != nil {
					slog.Error("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail)", slog.Any("error", err))
				}

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				err = mint.MintDB.ChangeLiquiditySwapState(lnStatusTx, swapRequest.Id, utils.LightningPaymentFail)
				if err != nil {
					slog.Error("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail)", slog.Any("error", err))
				}
			}
			err = lnStatusTx.Commit(ctx)
			if err != nil {
				return
			}
			return
		}

		txSwap, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			slog.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		swapRequest, err = mint.MintDB.GetLiquiditySwapById(txSwap, swapId)
		if err != nil {
			c.Error(errors.New("mint.MintDB.GetLiquiditySwapById(swapId)"))
			return
		}
		swapRequest.State = utils.Finished

		err = mint.MintDB.ChangeLiquiditySwapState(txSwap, swapRequest.Id, swapRequest.State)
		if err != nil {
			slog.Error("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail)", slog.Any("error", err))
		}

		err = txSwap.Commit(ctx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			slog.Error("Failed to commit transaction", slog.Any("error", err))
			return
		}

		// change swap to waiting for chain confirmations
		component := templates.SwapState(swapRequest.State, swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}
	}
}
func generateQR(data string) (string, error) {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return "", err
	}
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

func LiquiditySummaryComponent(handler *adminHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		balance, err := handler.EcashBalance(time.Unix(0, 0))
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetNeededSats(). %w", err))
			return
		}

		lnSatsBalance, err := handler.lnSatsBalance()
		if err != nil {
			c.Error(fmt.Errorf("handler.lnSatsBalance(). %w", err))
			return
		}

		component := templates.LiquiditySummaryComponent(lnSatsBalance, balance)
		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return

		}
	}
}
