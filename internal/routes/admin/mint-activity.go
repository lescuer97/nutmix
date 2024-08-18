package admin

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
)

func MintActivityTab(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		c.HTML(200, "mint-activity", mint.Config)
	}
}

func MintBalance(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		milillisatBalance, err := mint.LightningComs.WalletBalance()
		if err != nil {
			errorMessage := ErrorNotif{
				Error: "There was a problem getting the balance",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		c.HTML(200, "node-balance", milillisatBalance/1000)
	}
}
