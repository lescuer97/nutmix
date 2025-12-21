package admin

import (
	"context"
	"embed"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"net/http"

	"log/slog"
	"os"

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

	// Create token blacklist
	tokenBlacklist := NewTokenBlacklist()

	adminRoute.Use(ErrorHtmlMessageMiddleware())
	// I use the first active keyset as secret for jwt token signing
	adminRoute.Use(AuthMiddleware(loginKey.Serialize(), tokenBlacklist))

	adminHandler := newAdminHandler(mint)

	// PAGES SETUP
	// This is /admin pages
	adminRoute.GET("/login", LoginPage(mint, nostrPubkey != nil))

	newLiquidity := make(chan string)
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

		liquidityMangerRouter := adminRoute.Group("")
		liquidityMangerRouter.Use(liquidityManagerMiddleware(mint))
		liquidityMangerRouter.GET("/liquidity", LigthningLiquidityPage(mint))
		liquidityMangerRouter.GET("/liquidity-button", LiquidityButton(mint))
		liquidityMangerRouter.GET("/liquidity/:swapId", SwapStatusPage(mint))
		liquidityMangerRouter.GET("/swaps-list", SwapsList(mint))
		liquidityMangerRouter.GET("/ln-send", LnSendPage(mint))
		liquidityMangerRouter.GET("/ln-receive", LnReceivePage(mint))
		liquidityMangerRouter.GET("/liquid-swap-form", SwapOutForm(mint))
		liquidityMangerRouter.GET("/lightning-swap-form", LightningSwapForm())
		liquidityMangerRouter.POST("/out-swap-req", SwapOutRequest(mint))
		liquidityMangerRouter.POST("/in-swap-req", SwapInRequest(mint, newLiquidity))
		liquidityMangerRouter.GET("/liquidity-summary", LiquiditySummaryComponent(&adminHandler))
		liquidityMangerRouter.GET("/swap/:swapId", SwapStateCheck(mint))
		liquidityMangerRouter.POST("/swap/:swapId/confirm", ConfirmSwapOutTransaction(mint, newLiquidity))
		go CheckStatusOfLiquiditySwaps(mint, newLiquidity)
	}

}
func liquidityManagerMiddleware(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND) {
			slog.Debug("Liquidity manager is not available", slog.String("backend", string(mint.Config.MINT_LIGHTNING_BACKEND)))
			c.Status(404)
			return
		}
		c.Next()
	}
}
