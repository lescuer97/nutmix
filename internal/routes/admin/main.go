package admin

import (
	"context"
	"crypto/rand"
	"log"

	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

type ErrorNotif struct {
	Error string
}

func AdminRoutes(ctx context.Context, r *gin.Engine, mint *mint.Mint, logger *slog.Logger) {
	testPath := os.Getenv("TEST_PATH")
	if testPath != "" {
		r.Static("static", testPath+"static")
		r.LoadHTMLGlob(testPath + "templates/**")

	} else {
		r.Static("static", "internal/routes/admin/static")
		r.LoadHTMLGlob("internal/routes/admin/templates/**")

	}
	adminRoute := r.Group("/admin")

	loginKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		logger.Error(
			"secp256k1.GeneratePrivateKey()",
			slog.String(utils.LogExtraInfo, err.Error()),
		)
		log.Panicf("secp256k1.GeneratePrivateKey(). %+v", err)

	}

	// I use the first active keyset as secret for jwt token signing
	adminRoute.Use(AuthMiddleware(logger, loginKey.Serialize()))

	// PAGES SETUP
	// This is /admin
	adminRoute.GET("", InitPage(mint))
	adminRoute.GET("/keysets", KeysetsPage(mint))
	adminRoute.GET("/settings", MintSettingsPage(mint))
	adminRoute.GET("/login", LoginPage(logger, mint))
	adminRoute.GET("/bolt11", LightningNodePage(mint))

	// change routes
	adminRoute.POST("/login", Login(mint, logger, loginKey))
	adminRoute.POST("/mintsettings", MintSettingsForm(mint, logger))
	adminRoute.POST("/bolt11", Bolt11Post(mint, logger))
	adminRoute.POST("/rotate/sats", RotateSatsSeed(mint, logger))

	// fractional html components
	adminRoute.GET("/keysets-layout", KeysetsLayoutPage(mint, logger))
	adminRoute.GET("/lightningdata", LightningDataFormFields(mint))
	adminRoute.GET("/mint-balance", MintBalance(mint, logger))
	adminRoute.GET("/mint-melt-summary", MintMeltSummary(mint, logger))
	adminRoute.GET("/mint-melt-list", MintMeltList(mint, logger))
	adminRoute.GET("/logs", LogsTab(logger))

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

func LogsTab(logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {

		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		// read logs
		logsdir, err := utils.GetLogsDirectory()

		if err != nil {
			logger.Warn(
				"utils.GetLogsDirectory()",
				slog.String(utils.LogExtraInfo, err.Error()))

		}

		file, err := os.Open(logsdir + "/" + mint.LogFileName)
		defer file.Close()
		if err != nil {
			logger.Warn(
				"os.Open(logsdir ",
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "Could not get logs from mint",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		logs := utils.ParseLogFileByLevelAndTime(file, []slog.Level{slog.LevelWarn, slog.LevelError, slog.LevelInfo}, timeRequestDuration.RollBackFromNow())

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
