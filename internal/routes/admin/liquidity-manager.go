package admin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"

	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
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
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}

// swaps out of the mint
func LiquidSwapForm(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
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
		component := templates.LiquidSwapBoltzPostForm(balance)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}

// Swaps into the mint
func LightningSwapForm(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		component := templates.LightningSwapBoltz()

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}

func SwapToLiquidRequest(logger *slog.Logger, mint *m.Mint, sdk *breez_sdk_liquid.BindingLiquidSdk) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// need amount and liquid address
		amountStr := c.PostForm("amount")

		amount, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil {
			c.Error(errors.New("strconv.ParseUint(amountStr, 10, 64 )"))
			return
		}

		address := c.PostForm("address")

		res, err := LightningToLiquidSwap(amount, sdk)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n LightningToLiquidSwap(amount,sdk ) %+v \n", err)
			return
		}

		uuid := uuid.New().String()
		swap := utils.SwapRequest{
			Amount:      uint(amount),
			Destination: address,
			State:       utils.WaitingUserConfirmation,
			Id:          uuid,
			Type:        utils.LiquidityOut,
		}

		decodedInvoice, err := zpay32.Decode(res.Destination, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n zpay32.Decode(res.Destination) %+v \n", err)
			return
		}

		err = mint.MintDB.AddSwapRequest(swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			return
		}

		c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=liquid&id="+uuid)
		component := templates.LiquidSwapSummary(decodedInvoice.MilliSat.ToSatoshis().String(), string(amount), swap.Destination, uuid)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New(`templates.LiquidSwapSummary(decodedInvoice.MilliSat.ToSatoshis().String(), string(amount),  "test address", uuid)`))
			return
		}

		return
	}
}

func SwapToLightningRequest(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
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
		swap := utils.SwapRequest{
			Amount:      uint(amount),
			Destination: resp.PaymentRequest,
			State:       utils.WaitingUserConfirmation,
			Id:          uuid,
			Type:        utils.LiquidityIn,
		}

		err = mint.MintDB.AddSwapRequest(swap)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n Could not add swap request %+v \n", err)
			return
		}

		c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=lightning&id="+swap.Id)
		component := templates.LightningSwapSummary(string(swap.Amount), resp.PaymentRequest, swap.Id)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
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

		swapRequest, err := mint.MintDB.GetSwapRequestById(swapId)
		if err != nil {
			c.Error(errors.New("mint.MintDB.GetSwapRequestById(swapId)"))
			return
		}

		component := templates.SwapState(string(swapRequest.State), swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}
func ConfirmSwapTransaction(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// only needs the amount and we generate an invoice from the mint directly
		swapId := c.Param("swapId")

		swapRequest, err := mint.MintDB.GetSwapRequestById(swapId)
		if err != nil {
			c.Error(errors.New("mint.MintDB.GetSwapRequestById(swapId)"))
			return
		}

		// change swap to waiting for lightning payment
		err = mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.BoltzWaitingPayment)
		if err != nil {
			c.Error(errors.New("mint.MintDB.ChangeSwapRequestState(swapId, utils.BoltzWaitingPayment)"))
			return
		}

		decodedInvoice, err := zpay32.Decode(swapRequest.Destination, mint.LightningBackend.GetNetwork())
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			c.Error(fmt.Errorf("zpay32.Decode(res.Destination) %w", err))
			return
		}

		fee := uint64(float64(swapRequest.Amount) * 0.10)

		payment, err := mint.LightningBackend.PayInvoice(swapRequest.Destination, decodedInvoice, fee, false, 0)

		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN {
			logger.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, err := mint.LightningBackend.CheckPayed(swapRequest.Destination)

			// if error on checking payement we will save as pending and returns status
			if err != nil {

				err = mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.UnknownProblem)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return
			}

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				// change melt request state
				err = mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.UnknownProblem)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.UnknownProblem): %w", err).Error())
				}

				return

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				err = mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.LightnigPaymentFail)
				if err != nil {
					logger.Error(fmt.Errorf("mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
				}
				return
			}
		}

		err = mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.WaitingBoltzTXConfirmations)
		if err != nil {
			logger.Error(fmt.Errorf("mint.MintDB.ChangeSwapRequestState(swapRequest.Id, utils.LightnigPaymentFail): %w", err).Error())
		}

		// change swap to waiting for chain confirmations
		component := templates.SwapState(string(utils.WaitingBoltzTXConfirmations), swapId)

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}
