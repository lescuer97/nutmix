package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"

	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/breez/breez-sdk-liquid-go/breez_sdk_liquid"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/tyler-smith/go-bip39"
)

type ErrorNotif struct {
	Error string
}

func AdminRoutes(ctx context.Context, r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	testPath := os.Getenv("TEST_PATH")
	if testPath != "" {
		r.Static("static", testPath+"static")
		r.LoadHTMLGlob(testPath + "templates/**")

	} else {
		r.Static("static", "internal/routes/admin/static")
		r.LoadHTMLGlob("internal/routes/admin/templates/*.html")

	}
	adminRoute := r.Group("/admin")
	// I use the first active keyset as secret for jwt token signing
	adminRoute.Use(AuthMiddleware(logger, mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Serialize()))

	// PAGES SETUP
	// This is /admin pages
	adminRoute.GET("", InitPage(mint))
	adminRoute.GET("/keysets", KeysetsPage(mint))
	adminRoute.GET("/settings", MintSettingsPage(mint))
	adminRoute.GET("/login", LoginPage(logger, mint))
	adminRoute.GET("/bolt11", LightningNodePage(mint))
	adminRoute.GET("/liquidity", LigthningLiquidityPage(logger, mint))

	// change routes
	adminRoute.POST("/login", Login(mint, logger))
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
	adminRoute.GET("/swaps-list", SwapsList(mint, logger))

	// only have swap routes if liquidity manager is possible
	if utils.CanUseLiquidityManager(mint.LightningBackend.GetNetwork()) {
		apiKey := os.Getenv("BOLTZ_SDK_KEY")

		// // setup liquid sdk
		config, err := breez_sdk_liquid.DefaultConfig(utils.GetBreezLiquid(mint.LightningBackend.GetNetwork()), &apiKey)
		if err != nil {
			log.Panicf("breez_sdk_liquid.DefaultConfig(breez_sdk_liquid.LiquidNetworkMainnet). %+v", err)
		}

		// get nmonic from private key
		mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
		if mint_privkey == "" {
			log.Panicf("Mint private key not available")
		}
		decodedPrivKey, err := hex.DecodeString(mint_privkey)
		if err != nil {
			log.Panicf("hex.DecodeString(mint_privkey). %+v", err)
		}

		parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

		masterKey, err := m.MintPrivateKeyToBip32(parsedPrivateKey)
		if err != nil {
			log.Panicf("m.MintPrivateKeyToBip32(parsedPrivateKey). %+v", err)
		}

		// path for liquid
		liquidKey, err := masterKey.NewChildKey(hdkeychain.HardenedKeyStart + LiquidCoinType)
		if err != nil {
			log.Panicf("masterKey.NewChildKey(hdkeychain.HardenedKeyStart + LiquidCoinType). %+v", err)
		}

		mnemonic, err := bip39.NewMnemonic(liquidKey.Key)

		if err != nil {
			log.Panicf("bip39.NewMnemonic(liquidKey.Key). %+v", err)
		}

		connectRequest := breez_sdk_liquid.ConnectRequest{
			Config:   config,
			Mnemonic: mnemonic,
		}

		sdk, err := breez_sdk_liquid.Connect(connectRequest)
		if err != nil {
			log.Panicf("breez_sdk_liquid.Connect(connectRequest). %+v", err)
		}
		// defer sdk.Disconnect()
		// liquidity manager
		adminRoute.GET("/liquidity-button", LiquidityButton(logger))
		adminRoute.GET("/liquid-swap-form", LiquidSwapForm(logger, mint))
		adminRoute.GET("/lightning-swap-form", LightningSwapForm(logger))

		adminRoute.POST("/liquid-swap-req", SwapToLiquidRequest(logger, mint, sdk))
		adminRoute.POST("/lightning-swap-req", SwapToLightningRequest(logger, mint))

		adminRoute.GET("/swap/:swapId", SwapStateCheck(logger, mint))

		adminRoute.POST("/swap/:swapId/confirm", ConfirmSwapOutTransaction(logger, mint))
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
