package admin

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func MintSettingsPage(pool *pgxpool.Pool, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(200, "settings.html", mint.Config)
	}
}

func MintSettingsForm(pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

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

		pegInLimitStr := c.Request.PostFormValue("PEG_IN_LIMIT_SATS")
		switch pegInLimitStr {

		case "":

			mint.Config.PEG_IN_LIMIT_SATS = nil
		default:

			pegInLimit, err := strconv.Atoi(pegInLimitStr)
			if err != nil {
				logger.Debug(
					"strconv.Atoi(pegInLimitStr)",
					slog.String(utils.LogExtraInfo, err.Error()))
				errorMessage := ErrorNotif{
					Error: "Peg in limit is not an integer",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			mint.Config.PEG_IN_LIMIT_SATS = &pegInLimit

		}

		// Check pegout limit.
		pegOutLimitStr := c.Request.PostFormValue("PEG_OUT_LIMIT_SATS")
		switch pegOutLimitStr {

		case "":
			mint.Config.PEG_OUT_LIMIT_SATS = nil
		default:
			pegOutLimit, err := strconv.Atoi(pegOutLimitStr)
			if err != nil {
				logger.Debug(
					"strconv.Atoi(pegInLimitStr)",
					slog.String(utils.LogExtraInfo, err.Error()))
				errorMessage := ErrorNotif{
					Error: "Peg out limit is not an integer",
				}
				c.HTML(200, "settings-error", errorMessage)
				return
			}

			mint.Config.PEG_OUT_LIMIT_SATS = &pegOutLimit

		}

		nostrKey := c.Request.PostFormValue("NOSTR")

		if len(nostrKey) > 0 {

			_, key, err := nip19.Decode(nostrKey)

			if err != nil {
				logger.Warn(
					"nip19.Decode(nostrKey)",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{
					Error: "Nostr npub is not valid",
				}

				c.HTML(200, "settings-error", errorMessage)

				return

			}

			switch nostr.IsValid32ByteHex(key.(string)) {
			case true:
				mint.Config.NOSTR = nostrKey
			case false:
				logger.Warn("Nostr npub is not valid")
				errorMessage := ErrorNotif{
					Error: "Nostr npub is not valid",
				}

				c.HTML(200, "settings-error", errorMessage)

				return

			}

		} else {
			mint.Config.NOSTR = ""

		}

		err := mint.Config.SetTOMLFile()
		if err != nil {
			logger.Error(
				"mint.Config.SetTOMLFile()",
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "there was a problem in the server",
			}

			c.HTML(200, "settings-error", errorMessage)

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

func LightningNodePage(pool *pgxpool.Pool, mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(200, "bolt11.html", mint.Config)
	}
}

func Bolt11Post(pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

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

		case string(m.FAKE_WALLET):

			mint.Config.MINT_LIGHTNING_BACKEND = m.FAKE_WALLET

			fakeWalletBackend := lightning.FakeWallet{
				Network: chainparam,
			}

			mint.LightningBackend = fakeWalletBackend
		case string(m.LNDGRPC):
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
			mint.Config.MINT_LIGHTNING_BACKEND = m.LNDGRPC
			mint.Config.LND_GRPC_HOST = lndHost
			mint.Config.LND_MACAROON = macaroon
			mint.Config.LND_TLS_CERT = tlsCert

		case string(m.LNBITS):
			lnbitsKey := c.Request.PostFormValue("MINT_LNBITS_KEY")
			lnbitsEndpoint := c.Request.PostFormValue("MINT_LNBITS_ENDPOINT")

			lnbitsWallet := lightning.LnbitsWallet{
				Network:  chainparam,
				Key:      lnbitsKey,
				Endpoint: lnbitsEndpoint,
			}
			mint.LightningBackend = lnbitsWallet
			mint.Config.MINT_LIGHTNING_BACKEND = m.LNBITS
			mint.Config.MINT_LNBITS_KEY = lnbitsKey
			mint.Config.MINT_LNBITS_ENDPOINT = lnbitsEndpoint
		case string(m.CLNGRPC):
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
			mint.Config.MINT_LIGHTNING_BACKEND = m.CLNGRPC
			mint.Config.CLN_GRPC_HOST = clnHost
			mint.Config.CLN_MACAROON = macaroon
			mint.Config.CLN_CA_CERT = clnCaCert
			mint.Config.CLN_CLIENT_KEY = clnClientKey
			mint.Config.CLN_CLIENT_CERT = clnClientCert
		}

		err = mint.Config.SetTOMLFile()
		if err != nil {
			logger.Error(
				"mint.Config.SetTOMLFile()",
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "There was a problem setting your config",
			}

			c.HTML(200, "settings-error", errorMessage)
			return

		}

		c.HTML(200, "settings-success", successMessage)
		return
	}
}
