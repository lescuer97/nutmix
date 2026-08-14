package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/nbd-wtf/go-nostr"
)

func setupMiddlewareEngine(secure bool) (*gin.Engine, *[]bool, []byte) {
	gin.SetMode(gin.TestMode)
	secret := []byte("test-secret")
	reached := &[]bool{}
	r := gin.New()
	r.Use(AuthMiddleware(secret, NewTokenBlacklist(), secure))
	r.GET("/admin/protected", func(c *gin.Context) {
		*reached = append(*reached, true)
		c.Status(http.StatusOK)
	})
	return r, reached, secret
}

func TestMakeJWTTokenHasExpiry(t *testing.T) {
	secret := []byte("test-secret")
	tokenStr, err := makeJWTToken(secret)
	if err != nil {
		t.Fatalf("makeJWTToken: %+v", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse: %+v", err)
	}

	exp, err := token.Claims.GetExpirationTime()
	if err != nil || exp == nil {
		t.Fatalf("expected exp claim, got err=%v exp=%v", err, exp)
	}
	if iat, _ := token.Claims.GetIssuedAt(); iat == nil {
		t.Fatal("expected iat claim to be set")
	}

	want := time.Now().Add(sessionTTL)
	if exp.Before(want.Add(-time.Minute)) || exp.After(want.Add(time.Minute)) {
		t.Errorf("exp %v not within a minute of expected %v", exp, want)
	}
}

func TestValidNostrLoginEvent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	validEvent := nostr.Event{Kind: adminLoginEventKind, CreatedAt: nostr.Timestamp(now.Unix())} //nolint:exhaustruct
	validLogin := database.NostrLoginAuth{Expiry: int(now.Add(adminLoginChallengeTTL).Unix())}   //nolint:exhaustruct

	tests := []struct {
		name  string
		event nostr.Event
		login database.NostrLoginAuth
		want  bool
	}{
		{name: "valid", event: validEvent, login: validLogin, want: true},
		{name: "wrong kind", event: nostr.Event{Kind: 1, CreatedAt: validEvent.CreatedAt}, login: validLogin, want: false},                                                                      //nolint:exhaustruct
		{name: "expired challenge", event: validEvent, login: database.NostrLoginAuth{Expiry: int(now.Add(-time.Second).Unix())}, want: false},                                                  //nolint:exhaustruct
		{name: "stale event", event: nostr.Event{Kind: adminLoginEventKind, CreatedAt: nostr.Timestamp(now.Add(-adminLoginChallengeTTL - time.Second).Unix())}, login: validLogin, want: false}, //nolint:exhaustruct
		{name: "future event", event: nostr.Event{Kind: adminLoginEventKind, CreatedAt: nostr.Timestamp(now.Add(adminLoginFutureSkew + time.Second).Unix())}, login: validLogin, want: false},   //nolint:exhaustruct
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validNostrLoginEvent(test.event, test.login, now); got != test.want {
				t.Errorf("validNostrLoginEvent() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAuthMiddlewareRejectsExpiredToken(t *testing.T) {
	r, reached, secret := setupMiddlewareEngine(false)

	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{ //nolint:exhaustruct
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * sessionTTL)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-sessionTTL)),
	}).SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString: %+v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/protected", nil)
	req.AddCookie(&http.Cookie{Name: AdminAuthKey, Value: expired}) //nolint:exhaustruct
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(*reached) != 0 {
		t.Error("protected handler was reached with an expired token")
	}
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect to login, got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsFreshToken(t *testing.T) {
	r, reached, secret := setupMiddlewareEngine(false)

	token, err := makeJWTToken(secret)
	if err != nil {
		t.Fatalf("makeJWTToken: %+v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/protected", nil)
	req.AddCookie(&http.Cookie{Name: AdminAuthKey, Value: token}) //nolint:exhaustruct
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(*reached) != 1 {
		t.Error("protected handler was not reached with a fresh token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleUnauthorizedSecureCookieFlag(t *testing.T) {
	for _, secure := range []bool{true, false} {
		r, _, _ := setupMiddlewareEngine(secure)

		req := httptest.NewRequest(http.MethodGet, "/admin/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		setCookie := w.Header().Get("Set-Cookie")
		hasSecure := strings.Contains(setCookie, "Secure")
		if hasSecure != secure {
			t.Errorf("secure=%v: Set-Cookie %q has Secure=%v", secure, setCookie, hasSecure)
		}
	}
}
