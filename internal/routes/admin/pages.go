package admin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

type LoginParams struct {
	Nonce     string
	ADMINNPUB string
}

func LoginPage(logger *slog.Logger, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		// generate nonce for login nostr
		nonce, err := cashu.GenerateNonceHex()
		if err != nil {
			if c.ContentType() == gin.MIMEJSON {
				c.JSON(500, "there was a problem generating a nonce")
			} else {
				c.HTML(200, "error.html", nil)
			}
		}

		nostrLogin := database.NostrLoginAuth{
			Nonce:     nonce,
			Expiry:    int(cashu.ExpiryTimeMinUnit(15)),
			Activated: false,
		}

		err = mint.MintDB.SaveNostrAuth(nostrLogin)
		if err != nil {
			logger.Error(
				"database.SaveNostrLoginAuth(pool, nostrLogin)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if c.ContentType() == gin.MIMEJSON {
				c.JSON(500, "there was a problem generating a nonce")
			} else {
				c.HTML(200, "error.html", nil)
			}
			return

		}

		adminNPUB := os.Getenv("ADMIN_NOSTR_NPUB")

		loginValues := LoginParams{
			Nonce:     nostrLogin.Nonce,
			ADMINNPUB: adminNPUB,
		}

		if c.ContentType() == gin.MIMEJSON {
			c.JSON(200, loginValues)
		} else {
			c.HTML(200, "login.html", loginValues)
		}

	}
}

func InitPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		err := templates.MintActivityLayout(utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND)).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func LigthningLiquidityPage(logger *slog.Logger, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {

			logger.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
		amount := strconv.FormatUint(milillisatBalance/1000, 10)

		err = templates.LiquidityDashboard(c.Query("swapForm"), amount).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func SwapStatusPage(logger *slog.Logger, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		swapId := c.Param("swapId")
		tx, err := mint.MintDB.GetTx(ctx)

		if err != nil {
			logger.Debug(
				"Incorrect body",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
				if err != nil {
					c.Error(fmt.Errorf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		swap, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			c.Error(err)
			return
		}
		amount := strconv.FormatUint(swap.Amount, 10)
		// generate qrCode
		qrcode, err := generateQR(swap.LightningInvoice)
		if err != nil {
			c.Error(fmt.Errorf("generateQR(swap.LightningInvoice). %w", err))
			return
		}

		var component templ.Component
		switch swap.Type {
		case utils.LiquidityIn:
			component = templates.LightningReceiveSummary(amount, swap.LightningInvoice, qrcode, swap.Id)
		case utils.LiquidityOut:
			component = templates.LightningSendSummary(amount, swap.LightningInvoice, swap.Id)

		}

		err = templates.SwapStatusPage(component).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}
