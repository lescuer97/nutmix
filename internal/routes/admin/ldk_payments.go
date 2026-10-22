package admin

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

var ldkPaymentsPageLoader = func(backend *ldk.LDK, paymentType ldk.PaymentType, cursor string) ([]ldk_node.PaymentDetails, string, error) {
	return backend.PaymentsPage(paymentType, cursor)
}

func LdkPaymentsFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, err := loadLdkPaymentsRequest(m, c.Query("type"), c.Query("cursor"))
		if err != nil {
			page = paymentsErrorPage(c.Query("type"), c.Query("cursor"), err)
		}
		renderLdkPaymentsPage(c, page)
	}
}

func loadLdkPaymentsRequest(m *mint.Mint, rawFilter string, cursor string) (templates.LdkPaymentsPage, error) {
	filter, err := parseLdkPaymentsFilter(rawFilter)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	backend, err := ldkBackendGetter(m)
	if err != nil {
		slog.Error("ldk backend type assertion failed", slog.String("event", "ldk_backend_type_assert_failed"), slog.String(utils.LogExtraInfo, err.Error()))
		return templates.LdkPaymentsPage{}, err
	}
	paymentType := ldk.All
	switch filter {
	case ldkPaymentsFilterIncoming:
		paymentType = ldk.Incoming
	case ldkPaymentsFilterOutgoing:
		paymentType = ldk.Outgoing
	}
	payments, next, err := ldkPaymentsPageLoader(backend, paymentType, cursor)
	if err != nil {
		slog.Error("could not fetch ldk payments", slog.String("event", "ldk_payments_fetch_failed"), slog.String(utils.LogExtraInfo, err.Error()))
		return templates.LdkPaymentsPage{}, err
	}
	return loadLdkPaymentsPage(payments, filter, cursor, next)
}

func paymentsErrorPage(rawFilter string, cursor string, err error) templates.LdkPaymentsPage {
	filter, filterErr := parseLdkPaymentsFilter(rawFilter)
	if filterErr != nil {
		return ldkPaymentsInvalidFilterPage()
	}
	if errors.Is(err, ldk_node.ErrNodeErrorInvalidPageToken) {
		page := newLdkPaymentsErrorPage("Invalid payment cursor")
		page.ActiveFilter = filter
		page.FirstURL = templates.LdkPaymentsURL(filter, "")
		page.RetryURL = templates.LdkPaymentsURL(filter, cursor)
		return page
	}
	page := ldkPaymentsLoadFailurePage()
	page.ActiveFilter = filter
	page.RetryURL = templates.LdkPaymentsURL(filter, cursor)
	if cursor != "" {
		page.FirstURL = templates.LdkPaymentsURL(filter, "")
	}
	return page
}

func renderLdkPaymentsPage(c *gin.Context, page templates.LdkPaymentsPage) {
	c.Status(200)
	if renderErr := templates.LdkPaymentsFragment(page).Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}
