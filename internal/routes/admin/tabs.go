package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	ErrInvalidNostrKey = errors.New("NOSTR npub is not valid")
)

func MintSettingsPage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		err := templates.MintSettings(mint.Config).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			c.Status(400)
			return
		}
		return
	}
}

func checkLimitSat(text string) (*int, error) {
	var finalInt *int = nil
	switch text {
	case "":

		return finalInt, nil
	default:
		pegInLimit, err := strconv.Atoi(text)
		if err != nil {
			return nil, fmt.Errorf("strconv.Atoi(text). %w", err)
		}
		finalInt = &pegInLimit
	}

	return finalInt, nil
}

func isNostrKeyValid(nostrKey string) (bool, error) {
	_, key, err := nip19.Decode(nostrKey)

	if err != nil {

		return false, fmt.Errorf("nip19.Decode(key): %w ", err)

	}

	return nostr.IsValid32ByteHex(key.(string)), nil

}

func changeAuthSettings(mint *m.Mint, c *gin.Context) error {
	activateAuthStr := c.Request.PostFormValue("MINT_REQUIRE_AUTH")
	activateAuth := false
	if activateAuthStr == "on" {
		activateAuth = true
	} else {
		activateAuth = false
	}

	oicdDiscoveryUrl := c.Request.PostFormValue("MINT_AUTH_OICD_DISCOVERY_URL")
	oicdClientId := c.Request.PostFormValue("MINT_AUTH_OICD_CLIENT_ID")
	rateLimitPerMinuteStr := c.Request.PostFormValue("MINT_AUTH_RATE_LIMIT_PER_MINUTE")
	maxBlindTokenStr := c.Request.PostFormValue("MINT_AUTH_MAX_BLIND_TOKENS")

	rateLimitPerMinute, err := strconv.ParseUint(rateLimitPerMinuteStr, 10, 64)
	if err != nil {
		return fmt.Errorf("strconv.ParseUint(rateLimitPerMinuteStr, 10, 64). %w", err)
	}
	maxBlindToken, err := strconv.ParseUint(maxBlindTokenStr, 10, 64)
	if err != nil {
		return fmt.Errorf("strconv.ParseUint(rateLimitPerMinuteStr, 10, 64). %w", err)
	}

	mint.Config.MINT_REQUIRE_AUTH = activateAuth
	mint.Config.MINT_AUTH_OICD_DISCOVERY_URL = oicdDiscoveryUrl
	mint.Config.MINT_AUTH_OICD_CLIENT_ID = oicdClientId
	mint.Config.MINT_AUTH_RATE_LIMIT_PER_MINUTE = int(rateLimitPerMinute)
	mint.Config.MINT_AUTH_MAX_BLIND_TOKENS = int32(maxBlindToken)

	return nil
}
func MintSettingsForm(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		// check the different variables that could change
		mint.Config.NAME = c.Request.PostFormValue("NAME")
		mint.Config.DESCRIPTION = c.Request.PostFormValue("DESCRIPTION")
		mint.Config.DESCRIPTION_LONG = c.Request.PostFormValue("DESCRIPTION_LONG")
		mint.Config.EMAIL = c.Request.PostFormValue("EMAIL")
		mint.Config.MOTD = c.Request.PostFormValue("MOTD")

		pegoutOnly := c.Request.PostFormValue("PEG_OUT_ONLY")
		if pegoutOnly == "on" {
			mint.Config.PEG_OUT_ONLY = true

		} else {
			mint.Config.PEG_OUT_ONLY = false
		}

		// Check pegin limit.
		pegInLitmit, err := checkLimitSat(c.Request.PostFormValue("PEG_IN_LIMIT_SATS"))
		if err != nil {
			logger.Debug(
				`checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))`,
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "peg out limit has a problem",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		mint.Config.PEG_IN_LIMIT_SATS = pegInLitmit

		// Check pegout limit.
		pegOutLitmit, err := checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))
		if err != nil {
			logger.Debug(
				`checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))`,
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "peg out limit has a problem",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		mint.Config.PEG_OUT_LIMIT_SATS = pegOutLitmit

		nostrKey := c.Request.PostFormValue("NOSTR")

		if len(nostrKey) > 0 {

			isValid, err := isNostrKeyValid(nostrKey)

			if err != nil {
				c.Error(ErrInvalidNostrKey)
				logger.Warn(
					"nip19.Decode(nostrKey)",
					slog.String(utils.LogExtraInfo, err.Error()))
				return
			}

			if !isValid {
				c.Error(ErrInvalidNostrKey)
				return
			}

			mint.Config.NOSTR = nostrKey
		} else {
			mint.Config.NOSTR = ""
		}

		err = changeAuthSettings(mint, c)
		if err != nil {
			c.Error(fmt.Errorf("changeAuthSettings(mint, c). %w", err))
			logger.Warn(
				`fmt.Errorf("changeAuthSettings(mint, c). %w", err)`,
				slog.String(utils.LogExtraInfo, err.Error()))
			return
		}

		successMessage := struct {
			Success string
		}{
			Success: "Settings successfully set",
		}

		c.HTML(200, "settings-success", successMessage)
	}
}

func LightningNodePage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(200, "bolt11.html", mint.Config)
	}
}

func Bolt11Post(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {

		successMessage := struct {
			Success string
		}{
			Success: "Lighning node settings changed successfully set",
		}

		formNetwork := c.Request.PostFormValue("NETWORK")

		chainparam, err := m.CheckChainParams(formNetwork)
		if err != nil {
			logger.Error(
				"m.CheckChainParams(formNetwork)",
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "Could not setup network for lightning",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		mint.Config.NETWORK = chainparam.Name

		switch c.Request.PostFormValue("MINT_LIGHTNING_BACKEND") {

		case string(utils.FAKE_WALLET):

			mint.Config.MINT_LIGHTNING_BACKEND = utils.FAKE_WALLET

			fakeWalletBackend := lightning.FakeWallet{
				Network: chainparam,
			}

			mint.LightningBackend = fakeWalletBackend
		case string(utils.LNDGRPC):
			lndHost := c.Request.PostFormValue("LND_GRPC_HOST")
			tlsCert := c.Request.PostFormValue("LND_TLS_CERT")
			macaroon := c.Request.PostFormValue("LND_MACAROON")

			lndWallet := lightning.LndGrpcWallet{
				Network: chainparam,
			}

			err := lndWallet.SetupGrpc(lndHost, macaroon, tlsCert)
			if err != nil {
				logger.Error(
					"lndWallet.SetupGrpc",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{
					Error: "Something went wrong setting up LND communications",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			// check connection
			_, err = lndWallet.WalletBalance()
			if err != nil {
				logger.Warn(
					"Could not get lightning balance",
					slog.String(utils.LogExtraInfo, err.Error()))
				errorMessage := ErrorNotif{
					Error: "Could not check stablished connection with Node",
				}

				c.HTML(200, "settings-error", errorMessage)
				return

			}
			mint.LightningBackend = lndWallet
			mint.Config.MINT_LIGHTNING_BACKEND = utils.LNDGRPC
			mint.Config.LND_GRPC_HOST = lndHost
			mint.Config.LND_MACAROON = macaroon
			mint.Config.LND_TLS_CERT = tlsCert

		case string(utils.LNBITS):
			lnbitsKey := c.Request.PostFormValue("MINT_LNBITS_KEY")
			lnbitsEndpoint := c.Request.PostFormValue("MINT_LNBITS_ENDPOINT")

			lnbitsWallet := lightning.LnbitsWallet{
				Network:  chainparam,
				Key:      lnbitsKey,
				Endpoint: lnbitsEndpoint,
			}
			mint.LightningBackend = lnbitsWallet
			mint.Config.MINT_LIGHTNING_BACKEND = utils.LNBITS
			mint.Config.MINT_LNBITS_KEY = lnbitsKey
			mint.Config.MINT_LNBITS_ENDPOINT = lnbitsEndpoint
		case string(utils.CLNGRPC):
			clnHost := c.Request.PostFormValue("CLN_GRPC_HOST")
			clnCaCert := c.Request.PostFormValue("CLN_CA_CERT")
			clnClientCert := c.Request.PostFormValue("CLN_CLIENT_CERT")
			clnClientKey := c.Request.PostFormValue("CLN_CLIENT_KEY")
			macaroon := c.Request.PostFormValue("CLN_MACAROON")

			clnWallet := lightning.CLNGRPCWallet{
				Network: chainparam,
			}

			err := clnWallet.SetupGrpc(clnHost, clnCaCert, clnClientCert, clnClientKey, macaroon)
			if err != nil {
				logger.Error(
					"lndWallet.SetupGrpc",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{
					Error: "Something went wrong setting up CLN communications",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			// check connection
			_, err = clnWallet.WalletBalance()
			if err != nil {
				logger.Warn(
					"Could not get lightning balance",
					slog.String(utils.LogExtraInfo, err.Error()))
				errorMessage := ErrorNotif{
					Error: "Could not check stablished connection with Node",
				}

				c.HTML(200, "settings-error", errorMessage)
				return

			}
			mint.LightningBackend = clnWallet
			mint.Config.MINT_LIGHTNING_BACKEND = utils.CLNGRPC
			mint.Config.CLN_GRPC_HOST = clnHost
			mint.Config.CLN_MACAROON = macaroon
			mint.Config.CLN_CA_CERT = clnCaCert
			mint.Config.CLN_CLIENT_KEY = clnClientKey
			mint.Config.CLN_CLIENT_CERT = clnClientCert
		}

		err = mint.MintDB.UpdateConfig(mint.Config)

		if err != nil {
			logger.Error(
				"mint.MintDB.UpdateConfig(mint.Config)",
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "there was a problem in the server",
			}

			c.HTML(200, "settings-error", errorMessage)

			return

		}

		c.HTML(200, "settings-success", successMessage)
		return
	}
}
