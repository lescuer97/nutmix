package admin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"

	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

type SwapState string

const PaidLightningToSDK SwapState = "PaidLightningToSDK"
const TransactionInMempool SwapState = "TransactionInMempool"
const SdkWalletWaitingForConfirmation SwapState = "SdkWalletWaitingForConfirmation"
const WaitingUserConfirmation SwapState = "WaitingUserConfirmation"
const WaitingForPayment SwapState = "WaitingForPayment"
const Paid SwapState = "Paid"

type SwapType string

const LiquidityOut SwapType = "LiquidityOut"
const LiquidityIn SwapType = "LiquidityIn"

type SwapRequest struct {
	Amount      uint      `json"amount"`
	Id          string    `json"id"`
	Destination string    `json"destination"`
	State       SwapState `json"state"`
	Type        SwapType  `json"type"`
}

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
		_ = SwapRequest{
			Amount:      uint(amount),
			Destination: address,
			State:       WaitingUserConfirmation,
			Id:          uuid,
			Type:        LiquidityOut,
		}

		decodedInvoice, err := zpay32.Decode(res.Destination, &chaincfg.TestNet3Params)
		if err != nil {
			// If the fees are acceptable, continue to create the Receive Payment
			log.Printf("\n zpay32.Decode(res.Destination) %+v \n", err)
			return
		}

		c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=liquid&id="+uuid)
		component := templates.LiquidSwapSummary(decodedInvoice.MilliSat.ToSatoshis().String(), string(amount), "test address", uuid)

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
		_ = SwapRequest{
			Amount:      uint(amount),
			Destination: resp.PaymentRequest,
			State:       WaitingUserConfirmation,
			Id:          uuid,
			Type:        LiquidityIn,
		}

		c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=lightning&id="+"4567")
		component := templates.LightningSwapSummary("10001", "10000", resp.PaymentRequest, "12345")

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
		_ = c.Param("swapId")

		component := templates.SwapState(string(WaitingUserConfirmation), "1234")

		err := component.Render(ctx, c.Writer)
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
		_ = c.Param("swapId")

		component := templates.SwapState(string(PaidLightningToSDK), "1234")

		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}
