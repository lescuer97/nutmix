package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

const lightningStatusTimeout = 5 * time.Second

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

func LightningBackendStatus(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeStatus := lightning.UNKNOWN_STATUS
		deprecated := false
		backend := mint.LightningBackend
		if backend != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), lightningStatusTimeout)
			defer cancel()

			status, err := backend.Status(ctx)
			if err != nil {
				slog.Warn("could not check lightning backend status", slog.Any("error", err))
				nodeStatus = lightning.OFFLINE_STATUS
			} else {
				nodeStatus = status
			}

			switch backend.LightningType() {
			case lightning.LNBITS, lightning.STRIKE: //nolint:staticcheck // Deprecated backends remain supported until their scheduled removal.
				deprecated = true
			}
		}

		switch nodeStatus {
		case lightning.ONLINE_STATUS, lightning.OFFLINE_STATUS, lightning.UNKNOWN_STATUS:
		default:
			nodeStatus = lightning.UNKNOWN_STATUS
		}

		if err := templates.LightningBackendStatus(nodeStatus, deprecated).Render(c.Request.Context(), c.Writer); err != nil {
			_ = c.Error(fmt.Errorf("templates.LightningBackendStatus(nodeStatus, deprecated).Render: %w", err))
		}
	}
}
