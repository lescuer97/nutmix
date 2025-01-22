package admin

import (
	"context"
	"encoding/hex"
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

			c.Error(err)
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

		err = mint.MintDB.AddLiquiditySwap(swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
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
			return
		}

		now := decodedInvoice.Timestamp.Add(decodedInvoice.Expiry()).Unix()
		swap.Expiration = uint64(now)

		err = mint.MintDB.AddLiquiditySwap(swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			return
		}

		amountConverted := strconv.FormatUint(swap.Amount, 10)

		c.Header("HX-Replace-URL", "/admin/liquidity/"+uuid)
		component := templates.LightningReceiveSummary(amountConverted, swap.LightningInvoice, swap.Id)

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

		swapRequest, err := mint.MintDB.GetLiquiditySwapById(swapId)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetLiquiditySwapById(swapId). %w", err))
			return
		}

		switch swapRequest.Type {
		case utils.LiquidityIn:
			decodedInvoice, err := zpay32.Decode(swapRequest.LightningInvoice, mint.LightningBackend.GetNetwork())
			if err != nil {
				// If the fees are acceptable, continue to create the Receive Payment
				c.Error(fmt.Errorf("zpay32.Decode(res.Destination, mint.LightningBackend.GetNetwork()). %w", err))
				return
			}

			payHash := hex.EncodeToString(decodedInvoice.PaymentHash[:])
			status, _, err := mint.LightningBackend.CheckPayed(payHash)
			if err != nil {
				c.Error(fmt.Errorf("mint.LightningBackend.CheckPayed(payHash). %w", err))
				return
			}

			switch status {
			case lightning.SETTLED:
				swapRequest.State = utils.Finished
			}
		case utils.LiquidityOut:

		}
		err = mint.MintDB.ChangeLiquiditySwapState(swapId, swapRequest.State)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapId, swapRequest.State). %w", err))
			return
		}

		component := templates.SwapState(string(swapRequest.State), swapId)

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

		swapRequest, err := mint.MintDB.GetLiquiditySwapById(swapId)
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

		logger.Info(fmt.Sprintf("making payment to breez for exchange. %+v", swapRequest.LightningInvoice))
		payment, err := mint.LightningBackend.PayInvoice(swapRequest.LightningInvoice, decodedInvoice, fee, false, 0)

		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN {
			logger.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, err := mint.LightningBackend.CheckPayed(swapRequest.LightningInvoice)

			// if error on checking payement we will save as pending and returns status
			if err != nil {

				err = mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return
			}

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				swapRequest.State = utils.LightnigPaymentPending
				// change melt request state
				err = mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				swapRequest.State = utils.LightnigPaymentFail
				err = mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, swapRequest.State)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
				}
				return
			}
		}

		swapRequest.State = utils.Finished

		err = mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, swapRequest.State)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.ChangeLiquiditySwapState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
		}

		// change swap to waiting for chain confirmations
		component := templates.SwapState(string(swapRequest.State), swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}
