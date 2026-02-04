package admin

import (
	"fmt"

	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func LightningDataFormFields(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		backend := c.Query(m.MINT_LIGHTNING_BACKEND_ENV)

		ctx := c.Request.Context()
		err := templates.SetupForms(backend, mint.Config).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(fmt.Errorf("templates.SetupForms(mint.Config).Render(ctx, c.Writer). %w", err))
			return
		}
	}
}
