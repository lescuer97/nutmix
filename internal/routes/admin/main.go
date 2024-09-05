package admin

import (
	"context"
	"crypto/rand"
	"log"
	"log/slog"
	"os"
	"slices"
	"time"

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

	// PAGES SETUP
	// This is /admin
	adminRoute.GET("", InitPage(ctx, pool, mint))
	adminRoute.GET("/keysets", KeysetsPage(ctx, pool, mint))
	adminRoute.GET("/settings", MintSettingsPage(ctx, pool, mint))
	adminRoute.GET("/login", LoginPage(ctx, pool, mint))
	adminRoute.GET("/bolt11", LightningNodePage(ctx, pool, mint))

	// change routes
	adminRoute.POST("/login", Login(ctx, pool, mint))
	adminRoute.POST("/mintsettings", MintSettingsForm(ctx, pool, mint))
	adminRoute.POST("/bolt11", Bolt11Post(ctx, pool, mint))
	adminRoute.POST("/rotate/sats", RotateSatsSeed(ctx, pool, mint))

	// fractional html components
	adminRoute.GET("/keysets-layout", KeysetsLayoutPage(ctx, pool, mint))
	adminRoute.GET("/lightningdata", LightningDataFormFields(ctx, pool, mint))
	adminRoute.GET("/mint-balance", MintBalance(ctx, pool, mint))
	adminRoute.GET("/mint-melt-summary", MintMeltSummary(ctx, pool, mint))
	adminRoute.GET("/mint-melt-list", MintMeltList(ctx, pool, mint))
	adminRoute.GET("/logs", LogsTab(ctx))

}

type TIME_REQUEST string

var (
	h24 TIME_REQUEST = "24h"
	h48 TIME_REQUEST = "48h"
	h72 TIME_REQUEST = "72h"
	d7  TIME_REQUEST = "7D"
	ALL TIME_REQUEST = "all"
)

func ParseToTimeRequest(str string) TIME_REQUEST {

	switch str {
	case "24h":
		return h24
	case "48h":
		return h48
	case "72h":
		return h72
	case "7d":
		return d7
	case "all":
		return ALL
	default:
		return h24
	}

}

// return 24 hours by default
func (t TIME_REQUEST) RollBackFromNow() time.Time {

	rollBackHour := time.Now()

	switch t {
	case h24:
		duration := time.Duration(24) * time.Hour
		return rollBackHour.Add(-duration)
	case h48:
		duration := time.Duration(48) * time.Hour
		return rollBackHour.Add(-duration)
	case h72:
		duration := time.Duration(72) * time.Hour
		return rollBackHour.Add(-duration)
	case d7:
		duration := time.Duration((7 * 24)) * time.Hour
		return rollBackHour.Add(-duration)
	case ALL:
		return time.Unix(1, 0)
	}
	duration := time.Duration(24) * time.Hour
	return rollBackHour.Add(-duration)
}

func LogsTab(ctx context.Context) gin.HandlerFunc {

	return func(c *gin.Context) {

		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		// read logs
		logsdir, err := utils.GetLogsDirectory()

		if err != nil {
			log.Panicln("Could not get Logs directory")
		}

		file, err := os.Open(logsdir + "/" + mint.LogFileName)
		defer file.Close()
		if err != nil {

			errorMessage := ErrorNotif{
				Error: "Could not get logs from mint",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		logs := utils.ParseLogFileByLevelAndTime(file, []slog.Level{slog.LevelWarn, slog.LevelError, slog.LevelInfo, slog.LevelDebug}, timeRequestDuration.RollBackFromNow())

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
