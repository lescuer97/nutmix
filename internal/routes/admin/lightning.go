package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	m "github.com/lescuer97/nutmix/internal/mint"
)

func LightningDataFormFields(pool *pgxpool.Pool, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		backend := c.Query(m.MINT_LIGHTNING_BACKEND_ENV)

		switch {

		case backend == string(m.LNDGRPC):
			c.HTML(200, "lnd-grpc-form", mint.Config)
			break
		case backend == string(m.CLNGRPC):
			c.HTML(200, "cln-grpc-form", mint.Config)
			break

		case backend == string(m.FAKE_WALLET):
			c.HTML(200, "fake-wallet-form", mint.Config)
			break

		case backend == string(m.LNBITS):
			c.HTML(200, "lnbits-wallet-form", mint.Config)
			break

		default:
			c.HTML(200, "problem-form", nil)

		}

		return
	}
}
