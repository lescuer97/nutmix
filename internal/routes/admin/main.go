package admin

import (
	"context"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

type ErrorNotif struct {
	Error string
}

func ErrorHtmlMessageMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			message := "Unknown Problem"
			for _, e := range c.Errors {
				switch {
				case errors.Is(e, utils.ErrAlreadyLNPaying):
					message = "Error paying invoice"
					return
				case errors.Is(e, ErrInvalidNostrKey):
					message = "Nostr npub is not valid"
					return
				case errors.Is(e, ErrUnitNotCorrect):
					message = "Keyset Unit is not correct"
					return
				}
			}
			logger.Error(fmt.Sprintf("Error from calls: %+v", c.Errors.String()))
			component := templates.ErrorNotif(message)
			err := component.Render(c.Request.Context(), c.Writer)
			if err != nil {
				logger.Error(fmt.Sprintf("could not render error notification: %+v", err))
				return
			}
		}

	}
}

//go:embed static/*.css static/*.js
var static embed.FS

//go:embed templates/*.html
var templatesFs embed.FS

func AdminRoutes(ctx context.Context, r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	contentStatic, err := fs.Sub(static, "static")
	if err != nil {
		logger.Error(
			`fs.Sub(static, "static")`,
			slog.String(utils.LogExtraInfo, err.Error()),
		)
		panic(err)
	}
	httpFs := http.FS(contentStatic)
	r.StaticFS("/static", httpFs)

	templ := template.Must(template.ParseFS(templatesFs, "templates/*.html"))
	r.SetHTMLTemplate(templ)

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
	// This is /admin pages
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

	// only have swap routes if liquidity manager is possible
	if utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND) {

		adminRoute.GET("/liquidity", LigthningLiquidityPage(logger, mint))
		adminRoute.GET("/liquidity/:swapId", SwapStatusPage(logger, mint))
		adminRoute.GET("/swaps-list", SwapsList(mint, logger))
		adminRoute.GET("/liquidity-button", LiquidityButton(logger))
		adminRoute.GET("/liquid-swap-form", SwapOutForm(logger, mint))
		adminRoute.GET("/lightning-swap-form", LightningSwapForm(logger))
		adminRoute.POST("/out-swap-req", SwapOutRequest(logger, mint))
		adminRoute.POST("/in-swap-req", SwapInRequest(logger, mint))
		adminRoute.GET("/swap/:swapId", SwapStateCheck(logger, mint))
		adminRoute.POST("/swap/:swapId/confirm", ConfirmSwapOutTransaction(logger, mint))
		go CheckStatusOfLiquiditySwaps(mint, logger)
	}

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

		file, err := os.Open(logsdir + "/" + m.LogFileName)
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
		ctx := context.Background()

		err = templates.Logs(logs).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
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
