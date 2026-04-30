package admin

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func setupAdminLDKRouteTestRouter(t *testing.T, backend utils.LightningBackend) *gin.Engine {
	t.Helper()

	mint := &m.Mint{}
	mint.Config = utils.Config{MINT_LIGHTNING_BACKEND: backend}

	r := gin.New()
	adminRoute := r.Group("/admin")
	ldkRoute := adminRoute.Group("")
	ldkRoute.Use(ldkNodeMiddleware(mint))
	ldkRoute.GET("/ldk", LdkNodePage(mint))
	ldkRoute.GET("/ldk/lightning", LdkLightningPage(mint))
	ldkRoute.GET("/ldk/payments", LdkPaymentsPage(mint))
	ldkRoute.GET("/ldk/onchain/address", LdkAddressFragment(mint))
	ldkRoute.GET("/ldk/onchain/balances", LdkBalancesFragment(mint))
	ldkRoute.GET("/ldk/onchain/send-form", LdkOnchainSendFormFragment(mint))
	ldkRoute.POST("/ldk/onchain/send", LdkSendOnchain(mint))
	ldkRoute.GET("/ldk/lightning/network-summary", LdkNetworkSummaryFragment(mint))
	ldkRoute.GET("/ldk/lightning/channel-form", LdkOpenChannelFormFragment(mint))
	ldkRoute.GET("/ldk/lightning/channels", LdkChannelsFragment(mint))
	ldkRoute.POST("/ldk/lightning/channels/open", LdkOpenChannel(mint))
	ldkRoute.POST("/ldk/lightning/channels/close", LdkCloseChannel(mint))
	ldkRoute.POST("/ldk/lightning/channels/force-close", LdkForceCloseChannel(mint))

	return r
}

func registeredRoutes(r *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{}, len(r.Routes()))
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}

func assertHasRoute(t *testing.T, routes map[string]struct{}, method string, path string) {
	t.Helper()

	key := method + " " + path
	if _, ok := routes[key]; !ok {
		t.Fatalf("expected route %s to be registered", key)
	}
}

func assertMissingRoute(t *testing.T, routes map[string]struct{}, method string, path string) {
	t.Helper()

	key := method + " " + path
	if _, ok := routes[key]; ok {
		t.Fatalf("did not expect route %s to be registered", key)
	}
}

func TestAdminLDKCanonicalRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	routes := registeredRoutes(setupAdminLDKRouteTestRouter(t, utils.LDK))

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/admin/ldk"},
		{method: http.MethodGet, path: "/admin/ldk/lightning"},
		{method: http.MethodGet, path: "/admin/ldk/payments"},
		{method: http.MethodGet, path: "/admin/ldk/onchain/address"},
		{method: http.MethodGet, path: "/admin/ldk/onchain/balances"},
		{method: http.MethodGet, path: "/admin/ldk/onchain/send-form"},
		{method: http.MethodPost, path: "/admin/ldk/onchain/send"},
		{method: http.MethodGet, path: "/admin/ldk/lightning/network-summary"},
		{method: http.MethodGet, path: "/admin/ldk/lightning/channel-form"},
		{method: http.MethodGet, path: "/admin/ldk/lightning/channels"},
		{method: http.MethodPost, path: "/admin/ldk/lightning/channels/open"},
		{method: http.MethodPost, path: "/admin/ldk/lightning/channels/close"},
		{method: http.MethodPost, path: "/admin/ldk/lightning/channels/force-close"},
	} {
		assertHasRoute(t, routes, route.method, route.path)
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/admin/ldk/address"},
		{method: http.MethodGet, path: "/admin/ldk/balances"},
		{method: http.MethodGet, path: "/admin/ldk/send-form"},
		{method: http.MethodPost, path: "/admin/ldk/send"},
		{method: http.MethodGet, path: "/admin/ldk/network-summary"},
		{method: http.MethodGet, path: "/admin/ldk/channel-form"},
		{method: http.MethodGet, path: "/admin/ldk/channels"},
		{method: http.MethodPost, path: "/admin/ldk/channels/open"},
		{method: http.MethodPost, path: "/admin/ldk/channels/close"},
		{method: http.MethodPost, path: "/admin/ldk/channels/force-close"},
	} {
		assertMissingRoute(t, routes, route.method, route.path)
	}
}
