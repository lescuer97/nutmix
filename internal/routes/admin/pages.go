package admin

import (
	"context"
	"log/slog"
	"os"

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

		err := templates.MintActivityLayout().Render(ctx, c.Writer)

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

		err = templates.LiquidityDashboard(c.Query("swapForm"), string(milillisatBalance/1000)).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}
