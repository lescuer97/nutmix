package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestAuthMiddlewareEnforcesCanonicalPatternsWithoutMethodRestriction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, method, path, pattern string
		clear                       bool
		wantCode                    int
	}{
		{name: "blind prefix", method: http.MethodGet, path: "/v1/mint/bolt11", pattern: "/v1/mint/*", clear: false, wantCode: http.StatusUnauthorized},
		{name: "blind melt", method: http.MethodPost, path: "/v1/melt/quote/bolt11", pattern: "/v1/melt/*", clear: false, wantCode: http.StatusUnauthorized},
		{name: "blind PUT remains protected", method: http.MethodPut, path: "/v1/mint/bolt11", pattern: "/v1/mint/*", clear: false, wantCode: http.StatusUnauthorized},
		{name: "clear prefix", method: http.MethodGet, path: "/v1/mint/bolt11", pattern: "/v1/mint/*", clear: true, wantCode: http.StatusUnauthorized},
		{name: "clear PUT remains protected", method: http.MethodPut, path: "/v1/mint/bolt11", pattern: "/v1/mint/*", clear: true, wantCode: http.StatusUnauthorized},
		{name: "blind unsupported regex", method: http.MethodGet, path: "/v1/mint/bolt11", pattern: "^/v1/(mint|melt)/.*", clear: false, wantCode: http.StatusInternalServerError},
		{name: "clear unsupported regex", method: http.MethodGet, path: "/v1/mint/bolt11", pattern: "^/v1/(mint|melt)/.*", clear: true, wantCode: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			config := utils.Config{MINT_REQUIRE_AUTH: true} //nolint:exhaustruct // auth middleware only needs auth settings
			if tc.clear {
				config.MINT_AUTH_CLEAR_AUTH_URLS = []string{tc.pattern}
			} else {
				config.MINT_AUTH_BLIND_AUTH_URLS = []string{tc.pattern}
			}
			m := &mint.Mint{Config: config} //nolint:exhaustruct // handlers only access config before rejecting missing auth
			r := gin.New()
			if tc.clear {
				r.Use(ClearAuthMiddleware(m))
			} else {
				r.Use(BlindAuthMiddleware(m))
			}
			r.Handle(tc.method, tc.path, func(c *gin.Context) { called = true; c.Status(http.StatusOK) })
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if called {
				t.Fatal("protected handler was called without auth header")
			}
		})
	}
}
