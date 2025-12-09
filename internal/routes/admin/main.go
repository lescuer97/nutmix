package admin

import (
	"context"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func ErrorHtmlMessageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			message := "Something went wrong"
			for _, e := range c.Errors {
				switch {
				case errors.Is(e, utils.ErrAlreadyLNPaying):
					message = "Error paying invoice"
				case errors.Is(e, ErrInvalidNostrKey):
					message = "Nostr npub is not valid"
				case errors.Is(e, ErrInvalidOICDURL):
					message = ErrInvalidOICDURL.Error()
				case errors.Is(e, ErrUnitNotCorrect):
					message = "Keyset Unit is not correct"
				case errors.Is(e, ErrInvalidStrikeCheck):
					message = ErrInvalidStrikeCheck.Error()
				case errors.Is(e, ErrInvalidStrikeConfig):
					message = ErrInvalidStrikeCheck.Error()
				case errors.Is(e, ErrIncorrectNpub):
					message = ErrIncorrectNpub.Error()
				case errors.Is(e, ErrCouldNotParseLogin):
					message = ErrCouldNotParseLogin.Error()
				case errors.Is(e, ErrInvalidNostrSignature):
					message = ErrInvalidNostrSignature.Error()
				}
			}
			slog.Error("Error from calls", slog.String("errors", c.Errors.String()))

			err := RenderError(c, message)
			if err != nil {
				slog.Error("Could not render error notification", slog.Any("error", err))
				return
			}
		}

	}
}

func renderHTMX(c *gin.Context, component templ.Component) error {
	c.Header("HX-Reswap", "innerHtml")
	c.Header("HX-Retarget", "#notifications")
	return component.Render(c.Request.Context(), c.Writer)
}

func RenderError(c *gin.Context, message string) error {
	return renderHTMX(c, templates.ErrorNotif(message))
}

func RenderSuccess(c *gin.Context, message string) error {
	return renderHTMX(c, templates.SuccessNotif(message))
}

//go:embed static/dist/js/*.js static/dist/js/modules/*.js static/dist/css/*.css
var staticEmbed embed.FS

func AdminRoutes(ctx context.Context, r *gin.Engine, mint *m.Mint) {
	// Create a file server for the embedded static files
	// The embed contains files at: static/dist/js/*.js and static/dist/css/*.css
	// We need to serve them at /js and /css routes
	jsFS, err := fs.Sub(staticEmbed, "static/dist/js")
	if err != nil {
		log.Panicf("could not create correct /dist/js directory")
	}
	cssFS, err := fs.Sub(staticEmbed, "static/dist/css")
	if err != nil {
		log.Panicf("could not create correct /dist/css directory")
	}

	r.StaticFS("/js", http.FS(jsFS))
	r.StaticFS("/css", http.FS(cssFS))

	adminRoute := r.Group("/admin")

	loginKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		slog.Error(
			"secp256k1.GeneratePrivateKey()",
			slog.String(utils.LogExtraInfo, err.Error()),
		)
		log.Panicf("secp256k1.GeneratePrivateKey(). %+v", err)
	}

	var nostrPubkey *btcec.PublicKey
	adminNpubStr := os.Getenv("ADMIN_NOSTR_NPUB")
	if adminNpubStr != "" {
		_, value, err := nip19.Decode(adminNpubStr)
		if err != nil {
			slog.Info("nip19.Decode(adminNpubStr)", slog.Any("error", err))
			panic("invalid  ADMIN_NOSTR_NPUB ")
		}

		decodedKey, err := hex.DecodeString(value.(string))
		if err != nil {
			slog.Info("hex.DecodeString(value.(string))", slog.Any("error", err))
			panic("decoded ADMIN_NOSTR_NPUB is not correct")
		}

		pubkey, err := schnorr.ParsePubKey(decodedKey)
		if err != nil {
			slog.Info("schnorr.ParsePubKey(decodedKey)", slog.Any("error", err))
			panic("")
		}

		nostrPubkey = pubkey
	}

	// INFO: if the admin page has a 404 we redirect to the login
	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() == http.StatusNotFound &&
			strings.Contains(c.Request.URL.Path, "/admin") {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
		}
	})
	// Create token blacklist
	tokenBlacklist := NewTokenBlacklist()

	adminRoute.Use(ErrorHtmlMessageMiddleware())
	// I use the first active keyset as secret for jwt token signing
	// adminRoute.Use(AuthMiddleware(loginKey.Serialize(), tokenBlacklist))

	adminHandler := newAdminHandler(mint)

	// PAGES SETUP
	// This is /admin pages
	adminRoute.GET("/login", LoginPage(mint, nostrPubkey != nil))

	fmt.Println("can use liquidity manager", utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND))
	if nostrPubkey != nil {
		adminRoute.GET("/summary", SummaryComponent(mint, &adminHandler))
		adminRoute.GET("/proofs-chart", ProofsChartCard(mint))
		adminRoute.GET("/api/proofs-chart-data", ProofsChartDataAPI(mint))
		adminRoute.GET("/blindsigs-chart", BlindSigsChartCard(mint))
		adminRoute.GET("/api/blindsigs-chart-data", BlindSigsChartDataAPI(mint))
		adminRoute.GET("", InitPage(mint))
		adminRoute.GET("/ln", LnPage(mint))
		adminRoute.GET("/ln-chart", LnChartCard(mint))
		adminRoute.GET("/api/ln-chart-data", LnChartDataAPI(mint))
		adminRoute.GET("/keysets", KeysetsPage(mint))
		adminRoute.GET("/settings", MintSettingsPage(mint))

		// change routes
		adminRoute.POST("/login", LoginPost(mint, loginKey, nostrPubkey))
		adminRoute.POST("/mintsettings/general", MintSettingsGeneral(mint))
		adminRoute.POST("/mintsettings/lightning", MintSettingsLightning(mint))
		adminRoute.POST("/mintsettings/auth", MintSettingsAuth(mint))
		// Legacy/Fallback
		adminRoute.POST("/bolt11", Bolt11Post(mint))
		adminRoute.POST("/rotate/sats", RotateSatsSeed(&adminHandler))
		adminRoute.POST("/logout", LogoutHandler(tokenBlacklist))

		// fractional html components
		adminRoute.GET("/keysets-layout", KeysetsLayoutPage(&adminHandler))
		adminRoute.GET("/lightningdata", LightningDataFormFields(mint))
		adminRoute.GET("/logs", LogsTab())

		// only have swap routes if liquidity manager is possible
		if utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND) {
			newLiquidity := make(chan string)

			adminRoute.GET("/liquidity", LigthningLiquidityPage(mint))
			adminRoute.GET("/liquidity-button", LiquidityButton(mint))
			adminRoute.GET("/liquidity/:swapId", SwapStatusPage(mint))
			adminRoute.GET("/swaps-list", SwapsList(mint))
			adminRoute.GET("/ln-send", LnSendPage(mint))
			adminRoute.GET("/ln-receive", LnReceivePage(mint))
			adminRoute.GET("/liquid-swap-form", SwapOutForm(mint))
			adminRoute.GET("/lightning-swap-form", LightningSwapForm())
			adminRoute.POST("/out-swap-req", SwapOutRequest(mint))
			adminRoute.POST("/in-swap-req", SwapInRequest(mint, newLiquidity))
			adminRoute.GET("/swap/:swapId", SwapStateCheck(mint))
			adminRoute.POST("/swap/:swapId/confirm", ConfirmSwapOutTransaction(mint, newLiquidity))
			go CheckStatusOfLiquiditySwaps(mint, newLiquidity)
		}
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

func LogsTab() gin.HandlerFunc {

	return func(c *gin.Context) {

		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		// read logs
		logsdir, err := utils.GetLogsDirectory()

		if err != nil {
			slog.Warn(
				"utils.GetLogsDirectory()",
				slog.String(utils.LogExtraInfo, err.Error()))

		}

		file, err := os.Open(logsdir + "/" + m.LogFileName)
		if err != nil {
			slog.Warn(
				"os.Open(logsdir ",
				slog.String(utils.LogExtraInfo, err.Error()))

			err := RenderError(c, "Could not get logs from mint")
			if err != nil {
				slog.Error("RenderError", slog.Any("error", err))
			}
			return
		}
		defer file.Close()

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
