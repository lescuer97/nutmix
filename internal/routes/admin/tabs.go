package admin

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
	"log"
)

func MintInfoTab(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		c.HTML(200, "mintinfo", mint.Config)
	}
}
func MintInfoPost(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		// c.Request.Form
		// check the different variables that could change
		mint.Config.NAME = c.Request.PostFormValue("NAME")
		mint.Config.DESCRIPTION = c.Request.PostFormValue("DESCRIPTION")
		mint.Config.DESCRIPTION_LONG = c.Request.PostFormValue("DESCRIPTION_LONG")
		mint.Config.EMAIL = c.Request.PostFormValue("EMAIL")
		mint.Config.NOSTR = c.Request.PostFormValue("NOSTR")
		mint.Config.MOTD = c.Request.PostFormValue("MOTD")

		err := mint.Config.SetTOMLFile()
		if err != nil {
			log.Println("mint.Config.SetTOMLFile() %w", err)
			errorMessage := ErrorNotif{
				Error: "there was a problem in the server",
			}

			c.HTML(200, "settings-error", errorMessage)
			return

		}

		successMessage := struct {
			Success string
		}{
			Success: "Settings successfully set",
		}

		c.HTML(200, "settings-success", successMessage)
	}
}
