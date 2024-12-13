package admin

import (
	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func LightningDataFormFields(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		backend := c.Query(m.MINT_LIGHTNING_BACKEND_ENV)

		switch {

		case backend == string(utils.LNDGRPC):
			c.HTML(200, "lnd-grpc-form", mint.Config)
			break
		case backend == string(utils.CLNGRPC):
			c.HTML(200, "cln-grpc-form", mint.Config)
			break

		case backend == string(utils.FAKE_WALLET):
			c.HTML(200, "fake-wallet-form", mint.Config)
			break

		case backend == string(utils.LNBITS):
			c.HTML(200, "lnbits-wallet-form", mint.Config)
			break

		default:
			c.HTML(200, "problem-form", nil)

		}

		return
	}
}
