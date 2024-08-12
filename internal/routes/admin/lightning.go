package admin

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/comms"
	m "github.com/lescuer97/nutmix/internal/mint"
	// "log"
)

func LightningDataFormFields(ctx context.Context, pool *pgxpool.Pool, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		backend := c.Query(m.MINT_LIGHTNING_BACKEND_ENV)

		switch {

		case backend == comms.LND_WALLET:
			c.HTML(200, "lnd-grpc-form", mint.Config)
			break

		case backend == comms.FAKE_WALLET:
			c.HTML(200, "fake-wallet-form", mint.Config)
			break

		case backend == comms.LNBITS_WALLET:
			c.HTML(200, "lnbits-wallet-form", mint.Config)
			break

		default:
			c.HTML(200, "problem-form", nil)

		}

		return
	}
}
