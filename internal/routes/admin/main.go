package admin

import (
	"context"
	"crypto/rand"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
)

const JWTSECRET = "JWTSECRET"

type ErrorNotif struct {
	Error string
}

func AdminRoutes(ctx context.Context, r *gin.Engine, pool *pgxpool.Pool, mint *mint.Mint) {
	r.Static("static", "internal/routes/admin/static")
	r.LoadHTMLGlob("internal/routes/admin/templates/**")
	adminRoute := r.Group("/admin")

	hmacSecret, err := generateHMACSecret()

	if err != nil {
		log.Panic("ERROR: could not create HMAC secret")
	}

	ctx = context.WithValue(ctx, JWTSECRET, hmacSecret)

	adminRoute.Use(AuthMiddleware(ctx))

	adminRoute.GET("", InitPage(ctx, pool, mint))
	adminRoute.GET("/login", LoginPage(ctx, pool, mint))
	adminRoute.POST("/login", Login(ctx, pool, mint))

	// partial template routes
	adminRoute.GET("/mintinfo", MintInfoTab(ctx, pool, mint))
	adminRoute.POST("/mintinfo", MintInfoPost(ctx, pool, mint))

	adminRoute.GET("/bolt11", Bolt11Tab(ctx, pool, mint))
	adminRoute.POST("/bolt11", Bolt11Post(ctx, pool, mint))

	adminRoute.GET("/keysets", KeysetsPage(ctx, pool, mint))

	adminRoute.POST("/rotate/sats", RotateSatsSeed(ctx, pool, mint))
	adminRoute.GET("/keysets-layout", KeysetsLayoutPage(ctx, pool, mint))

	adminRoute.GET("/lightningdata", LightningDataFormFields(ctx, pool, mint))

	adminRoute.GET("/mintactivity", MintActivityTab(ctx, pool, mint))
	adminRoute.GET("/mint-balance", MintBalance(ctx, pool, mint))

}

func generateHMACSecret() ([]byte, error) {
	// generate random Nonce
	secret := make([]byte, 32)  // create a slice with length 16 for the nonce
	_, err := rand.Read(secret) // read random bytes into the nonce slice
	if err != nil {
		return secret, err
	}

	return secret, nil
}
