package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
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

        balance := strconv.FormatUint( milillisatBalance / 1000, 10 )
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

func LiquidSwapRequest(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
        // need amount and liquid address 
        amountStr := c.PostForm("amount")

        _ ,err := strconv.ParseUint(amountStr, 10, 64 )
		if err != nil {
			c.Error(errors.New("strconv.ParseUint(amountStr, 10, 64 )"))
			return
		}

        _ = c.PostForm("address")

        c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=liquid&id=" + "4567")
		component := templates.LiquidSwapSummary("10001",  "test address", "4567")

		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New(`templates.LiquidSwapSummary("10001",  "test address")`))
			return
		}

		return
	}
}

func LightningSwapRequest(logger *slog.Logger, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

        // only needs the amount and we generate an invoice from the mint directly
        amountStr := c.PostForm("amount")

        amount ,err := strconv.ParseUint(amountStr, 10, 64 )
		if err != nil {
			c.Error(fmt.Errorf("strconv.ParseUint(amountStr, 10, 64 ). %w", err))
			return
		}

        resp, err := mint.LightningBackend.RequestInvoice(int64(amount))
		if err != nil {
			c.Error(fmt.Errorf("mint.LightningBackend.RequestInvoice(int64(amount)). %w", err))
			return
		}

        c.Header("HX-Replace-URL", "/admin/liquidity?swapForm=lightning&id=" + "4567")
		component := templates.LightningSwapSummary("10001",  resp.PaymentRequest, "12345")

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


		component := templates.SwapState("Not Paid")

        err := component.Render(ctx, c.Writer)
		if err != nil {
			c.Error(errors.New("component.Render(ctx, c.Writer)"))
			return
		}

		return
	}
}
