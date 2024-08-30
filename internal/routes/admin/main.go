package admin

import (
	"context"
	"crypto/rand"
	"log"
	"log/slog"
	"os"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

const JWT_SECRET = "JWT_SECRET"

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

	ctx = context.WithValue(ctx, JWT_SECRET, hmacSecret)

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
	adminRoute.GET("/mint-melt", MintMeltActivity(ctx, pool, mint))

	adminRoute.GET("/logs", LogsTab(ctx))

}
func LogsTab(ctx context.Context) gin.HandlerFunc {

	return func(c *gin.Context) {
		// read logs
		logsdir, err := utils.GetLogsDirectory()

		if err != nil {
			log.Panicln("Could not get Logs directory")
		}

		file, err := os.Open(logsdir + "/" + mint.LogFileName)
		if err != nil {

			errorMessage := ErrorNotif{
				Error: "Could not get logs from mint",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		logs := utils.ParseLogFileByLevel(file, []slog.Level{slog.LevelWarn, slog.LevelError, slog.LevelInfo})

		slices.Reverse(logs)

		c.HTML(200, "logs", logs)
	}
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
