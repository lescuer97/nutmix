package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
	qrcode "github.com/skip2/go-qrcode"
)

func LiquidityButton(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		component := templates.LiquidityButton()

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}

		return
	}
}

// swaps out of the mint
func SwapOutForm(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {

			logger.Warn(
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
func LightningSwapForm(logger *slog.Logger) gin.HandlerFunc {
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

func SwapOutRequest(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// need amount and liquid address
		invoice := c.PostForm("invoice")

		decodedInvoice, err := zpay32.Decode(invoice, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n zpay32.Decode(res.Destination) %+v \n", err)
			c.Error(fmt.Errorf("\n zpay32.Decode(res.Destination) %+v \n", err))
			return
		}

		uuid := uuid.New().String()
		swap := utils.LiquiditySwap{
			Amount:           uint64(decodedInvoice.MilliSat.ToSatoshis()),
			LightningInvoice: invoice,
			State:            utils.WaitingUserConfirmation,
			Id:               uuid,
			Type:             utils.LiquidityOut,
		}

		now := decodedInvoice.Timestamp.Add(decodedInvoice.Expiry()).Unix()
		swap.Expiration = uint64(now)

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			logger.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				logger.Error("\n Rolling back  because of failure %+v\n", p)
				tx.Rollback(ctx)
			} else if err != nil {
				logger.Error(fmt.Sprintf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
				if err != nil {
					logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		err = mint.MintDB.AddLiquiditySwap(tx, swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			c.Error(fmt.Errorf("\n Could not add swap request %+v \n", err))
			return
		}

		c.Header("HX-Replace-URL", "/admin/liquidity/"+uuid)
		component := templates.LightningSendSummary(decodedInvoice.MilliSat.ToSatoshis().Format(btcutil.AmountSatoshi), swap.LightningInvoice, uuid)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New(`templates.LiquidSwapSummary(decodedInvoice.MilliSat.ToSatoshis().String(), string(amount),  "test address", uuid)`))
			return
		}

		return
	}
}

func SwapInRequest(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// only needs the amount and we generate an invoice from the mint directly
		amountStr := c.PostForm("amount")

		amount, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil {
			c.Error(fmt.Errorf("strconv.ParseUint(amountStr, 10, 64 ). %w", err))
			return
		}

		resp, err := mint.LightningBackend.RequestInvoice(int64(amount))
		if err != nil {
			c.Error(fmt.Errorf("mint.LightningBackend.RequestInvoice(int64(amount)). %w", err))
			return
		}
		uuid := uuid.New().String()
		swap := utils.LiquiditySwap{
			Amount:           amount,
			LightningInvoice: resp.PaymentRequest,
			State:            utils.MintWaitingPaymentRecv,
			Id:               uuid,
			Type:             utils.LiquidityIn,
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
			logger.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				logger.Error("\n Rolling back  because of failure %+v\n", p)
				tx.Rollback(ctx)
			} else if err != nil {
				logger.Error(fmt.Sprintf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
				if err != nil {
					logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		err = mint.MintDB.AddLiquiditySwap(tx, swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			c.Error(fmt.Errorf("\n Could not add swap request %+v \n", err))
			return
		}

		amountConverted := strconv.FormatUint(swap.Amount, 10)

		// generate qrCode
		qrcode, err := generateQR(swap.LightningInvoice)
		if err != nil {
			c.Error(fmt.Errorf("generateQR(swap.LightningInvoice). %w", err))
			return
		}

		c.Header("HX-Replace-URL", "/admin/liquidity/"+uuid)
		component := templates.LightningReceiveSummary(amountConverted, swap.LightningInvoice, qrcode, swap.Id)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(fmt.Errorf("component.Render(ctx, c.Writer). %w", err))
			return
		}

		return
	}
}

func SwapStateCheck(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		// only needs the amount and we generate an invoice from the mint directly
		swapId := c.Param("swapId")

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			logger.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				logger.Error("\n Rolling back  because of failure %+v\n", p)
				tx.Rollback(ctx)
			} else if err != nil {
				logger.Error(fmt.Sprintf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
				if err != nil {
					logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		swapRequest, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {

			// pgx.Lo
			c.Error(fmt.Errorf("mint.MintDB.GetLiquiditySwapById(swapId). %w", err))
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

func ConfirmSwapOutTransaction(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// only needs the amount and we generate an invoice from the mint directly
		swapId := c.Param("swapId")

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			logger.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				logger.Error("\n Rolling back  because of failure %+v\n", p)
				tx.Rollback(ctx)
			} else if err != nil {
				logger.Error(fmt.Sprintf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
				if err != nil {
					logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
				}
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
		err = tx.Commit(ctx)
		if err != nil {
			logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
		}
		tx, err = mint.MintDB.GetTx(ctx)
		if err != nil {
			logger.Debug(
				"Could not get db transactions",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}
		swapRequest, err = mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			c.Error(errors.New("mint.MintDB.GetLiquiditySwapById(swapId)"))
			return
		}

		decodedInvoice, err := zpay32.Decode(swapRequest.LightningInvoice, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			c.Error(fmt.Errorf("zpay32.Decode(res.Destination) %w", err))
			return
		}

		fee := uint64(float64(swapRequest.Amount) * 0.10)

		logger.Info(fmt.Sprintf("making payment to invoice: %+v", swapRequest.LightningInvoice))
		payment, err := mint.LightningBackend.PayInvoice(swapRequest.LightningInvoice, decodedInvoice, fee, false, 0)

		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN {
			logger.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, _, err := mint.LightningBackend.CheckPayed(swapRequest.LightningInvoice)

			// if error on checking payement we will save as pending and returns status
			if err != nil {

				err = mint.MintDB.ChangeLiquiditySwapState(tx, swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return
			}

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				swapRequest.State = utils.LightningPaymentPending
				// change melt request state
				err = mint.MintDB.ChangeLiquiditySwapState(tx, swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				swapRequest.State = utils.LightningPaymentFail
				err = mint.MintDB.ChangeLiquiditySwapState(tx, swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
				}
				return
			}
		}

		swapRequest.State = utils.Finished

		err = mint.MintDB.ChangeLiquiditySwapState(tx, swapRequest.Id, swapRequest.State)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
		}

		// change swap to waiting for chain confirmations
		component := templates.SwapState(swapRequest.State, swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
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
