package admin

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

func LdkPaymentsFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			slog.Error("ldk backend type assertion failed",
				slog.String("event", "ldk_backend_type_assert_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkPaymentsPage(c, ldkPaymentsLoadFailurePage())
			return
		}
		paymentType := ldk.All
		queryType := c.Query("type")
		switch strings.TrimSpace(queryType) {
		case "incoming":
			paymentType = ldk.Incoming
		case "outgoing":
			paymentType = ldk.Outgoing

		}

		payments, err := ldkBackend.Payments(paymentType)
		if err != nil {
			slog.Error("could not fetch ldk payments",
				slog.String("event", "ldk_payments_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkPaymentsPage(c, ldkPaymentsLoadFailurePage())
			return
		}

		page, err := loadLdkPaymentsPage(payments, c.Query("type"), c.Query("show"))
		if err != nil {
			renderLdkPaymentsPage(c, ldkPaymentsPageForError(err))
			return
		}

		renderLdkPaymentsPage(c, page)
	}
}

func renderLdkPaymentsPage(c *gin.Context, page templates.LdkPaymentsPage) {
	c.Status(200)
	if renderErr := templates.LdkPaymentsFragment(page).Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}
