package admin

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
)

func LoginPage(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		// generate nonce for login nostr
		nonce, err := cashu.GenerateNonceHex()
		if err != nil {
			c.HTML(200, "error.html", nil)
		}

		nostrLogin := cashu.NostrLoginAuth{
			Nonce:     nonce,
			Expiry:    int(cashu.ExpiryTimeMinUnit(15)),
			Activated: false,
		}

		database.SaveNostrLoginAuth(pool, nostrLogin)

		adminNPUB := ctx.Value("ADMIN_NOSTR_NPUB").(string)

		loginValues := struct {
			Nonce     string
			ADMINNPUB string
		}{
			Nonce:     nostrLogin.Nonce,
			ADMINNPUB: adminNPUB,
		}

		c.HTML(200, "login.html", loginValues)
	}
}

func InitPage(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	}
}
