package admin

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"log"
	"strconv"
)

func MintInfoTab(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		c.HTML(200, "mint-settings", mint.Config)
	}
}
func MintInfoPost(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

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
			log.Println("mint.Config.SetTOMLFile() %w", err)
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
func Bolt11Tab(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		c.HTML(200, "bolt11-info", mint.Config)
	}
}
func Bolt11Post(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {

		successMessage := struct {
			Success string
		}{
			Success: "Lighning node settings changed successfully set",
		}

		// check if the the lightning values have change if yes try to setup a new connection client for mint
		mint.Config.NAME = c.Request.PostFormValue("NETWORK")

		switch c.Request.PostFormValue("MINT_LIGHTNING_BACKEND") {

		case comms.FAKE_WALLET:

			mint.Config.MINT_LIGHTNING_BACKEND = comms.FAKE_WALLET

			successMessage.Success = "Nothing to change"
			c.HTML(200, "settings-success", successMessage)
		case comms.LND_WALLET:

			lndHost := c.Request.PostFormValue("LND_GRPC_HOST")
			tlsCert := c.Request.PostFormValue("LND_TLS_CERT")
			macaroon := c.Request.PostFormValue("LND_MACAROON")

			if lndHost != mint.Config.LND_GRPC_HOST || tlsCert != mint.Config.LND_TLS_CERT || macaroon != mint.Config.LND_MACAROON {
				newCommsData := comms.LightingCommsData{
					MINT_LIGHTNING_BACKEND: comms.LND_WALLET,
					LND_GRPC_HOST:          lndHost,
					LND_TLS_CERT:           tlsCert,
					LND_MACAROON:           macaroon,
				}
				lightningComs, err := comms.SetupLightingComms(newCommsData)

				if err != nil {
					errorMessage := ErrorNotif{
						Error: "Something went wrong setting up LND communications",
					}

					c.HTML(200, "settings-error", errorMessage)
					return

				}

				// check connection
				_, err = lightningComs.WalletBalance()
				if err != nil /* || !validConnection */ {
					errorMessage := ErrorNotif{
						Error: "Could not check stablished connection with Node",
					}

					log.Printf("Error message %+v", errorMessage)

					c.HTML(200, "settings-error", errorMessage)
					return

				}
				mint.LightningComs = *lightningComs
				mint.Config.MINT_LIGHTNING_BACKEND = newCommsData.MINT_LIGHTNING_BACKEND
				mint.Config.LND_GRPC_HOST = newCommsData.LND_GRPC_HOST
				mint.Config.LND_MACAROON = newCommsData.LND_MACAROON
				mint.Config.LND_TLS_CERT = newCommsData.LND_TLS_CERT
				c.HTML(200, "settings-success", successMessage)

			} else {
				successMessage.Success = "Nothing to change"
				c.HTML(200, "settings-success", successMessage)

			}

		case comms.LNBITS_WALLET:
			lnbitsKey := c.Request.PostFormValue("MINT_LNBITS_KEY")
			lnbitsEndpoint := c.Request.PostFormValue("MINT_LNBITS_ENDPOINT")

			newCommsData := comms.LightingCommsData{
				MINT_LIGHTNING_BACKEND: comms.LNBITS_WALLET,
				MINT_LNBITS_ENDPOINT:   lnbitsEndpoint,
				MINT_LNBITS_KEY:        lnbitsKey,
			}
			lightningComs, err := comms.SetupLightingComms(newCommsData)

			if err != nil {
				errorMessage := ErrorNotif{
					Error: "Something went wrong setting up LNBITS communications",
				}

				c.HTML(200, "settings-error", errorMessage)
				return

			}

			// check connection
			_, err = lightningComs.WalletBalance()
			if err != nil {
				errorMessage := ErrorNotif{
					Error: "Could not check stablished connection with Node",
				}

				log.Printf("Error message %+v", errorMessage)

				c.HTML(200, "settings-error", errorMessage)
				return

			}
			mint.LightningComs = *lightningComs

			mint.Config.MINT_LIGHTNING_BACKEND = newCommsData.MINT_LIGHTNING_BACKEND
			mint.Config.MINT_LNBITS_KEY = newCommsData.MINT_LNBITS_KEY
			mint.Config.MINT_LNBITS_ENDPOINT = newCommsData.MINT_LNBITS_ENDPOINT
			c.HTML(200, "settings-success", successMessage)

		}

		err := mint.Config.SetTOMLFile()
		if err != nil {
			log.Println("mint.Config.SetTOMLFile() %w", err)
			errorMessage := ErrorNotif{
				Error: "there was a problem in the server",
			}

			c.HTML(200, "settings-error", errorMessage)
			return

		}

		return
	}
}
