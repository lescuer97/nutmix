package admin

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
)

type LoginParams struct {
	Nonce     string
	ADMINNPUB string
}

func LoginPage(pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
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

		nostrLogin := cashu.NostrLoginAuth{
			Nonce:     nonce,
			Expiry:    int(cashu.ExpiryTimeMinUnit(15)),
			Activated: false,
		}

		database.SaveNostrLoginAuth(pool, nostrLogin)

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

func InitPage(pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(200, "mint_activity.html", nil)
	}
}
