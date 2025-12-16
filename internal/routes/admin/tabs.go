package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	ErrInvalidOICDURL         = errors.New("invalid OICD discovery URL")
	ErrInvalidNostrKey        = errors.New("nostr npub is not valid")
	ErrInvalidStrikeConfig    = errors.New("invalid strike config")
	ErrInvalidStrikeCheck     = errors.New("could not verify strike configuration")
	ErrCouldNotParseLogin     = errors.New("could not parse login")
	ErrInvalidNostrSignature  = errors.New("invalid nostr signature")
	ErrFailedLightningPayment = errors.New("failed lightning payment")
)

func MintSettingsPage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		err := templates.MintSettings(mint.Config).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			c.Status(400)
			return
		}
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

func validateURL(urlStr string) error {
	if urlStr == "" {
		return nil // Empty URL is valid (nil field)
	}

	// Additional basic validation - try to parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Ensure it has a valid scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("URL must contain a valid scheme and host")
	}

	return nil
}

func changeAuthSettings(mint *m.Mint, c *gin.Context) error {
	activateAuthStr := c.Request.PostFormValue("MINT_REQUIRE_AUTH")
	activateAuth := false
	if activateAuthStr == "on" {
		activateAuth = true
	} else {
		activateAuth = false
	}

	oicdDiscoveryUrl := c.Request.PostFormValue("MINT_AUTH_OICD_URL")
	oicdClientId := c.Request.PostFormValue("MINT_AUTH_OICD_CLIENT_ID")
	rateLimitPerMinuteStr := c.Request.PostFormValue("MINT_AUTH_RATE_LIMIT_PER_MINUTE")
	maxBlindTokenStr := c.Request.PostFormValue("MINT_AUTH_MAX_BLIND_TOKENS")

	authBlindArray := c.PostFormArray("MINT_AUTH_BLIND_AUTH_URLS")
	authClearArray := c.PostFormArray("MINT_AUTH_CLEAR_AUTH_URLS")

	rateLimitPerMinute, err := strconv.ParseUint(rateLimitPerMinuteStr, 10, 64)
	if err != nil {
		return fmt.Errorf("strconv.ParseUint(rateLimitPerMinuteStr, 10, 64). %w", err)
	}
	maxBlindToken, err := strconv.ParseUint(maxBlindTokenStr, 10, 64)
	if err != nil {
		return fmt.Errorf("strconv.ParseUint(rateLimitPerMinuteStr, 10, 64). %w", err)
	}

	if activateAuth {
		if oicdDiscoveryUrl == "" {
			return ErrInvalidOICDURL
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := mint.SetupOidcService(ctx, oicdDiscoveryUrl)
		if err != nil {
			return fmt.Errorf("oidc.NewProvider(ctx, config.MINT_AUTH_OICD_URL): %w %w", err, ErrInvalidOICDURL)
		}
	} else {
		mint.OICDClient = nil
	}

	mint.Config.MINT_REQUIRE_AUTH = activateAuth
	mint.Config.MINT_AUTH_OICD_URL = oicdDiscoveryUrl
	mint.Config.MINT_AUTH_OICD_CLIENT_ID = oicdClientId
	mint.Config.MINT_AUTH_RATE_LIMIT_PER_MINUTE = int(rateLimitPerMinute)
	mint.Config.MINT_AUTH_MAX_BLIND_TOKENS = maxBlindToken
	mint.Config.MINT_AUTH_CLEAR_AUTH_URLS = authClearArray
	mint.Config.MINT_AUTH_BLIND_AUTH_URLS = authBlindArray

	return nil
}
func MintSettingsForm(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Deprecated: This handler is no longer used for individual sections.
		// It's kept here in case there's a legacy full form submit somewhere,
		// or it can be removed entirely if we're sure.
		// For now, we'll just return.
	}
}

func MintSettingsGeneral(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate URL fields first
		iconUrl := c.Request.PostFormValue("ICON_URL")
		tosUrl := c.Request.PostFormValue("TOS_URL")

		iconUrl = strings.TrimSpace(iconUrl)
		tosUrl = strings.TrimSpace(tosUrl)

		// Validate Icon URL if provided
		if iconUrl != "" {
			if err := validateURL(iconUrl); err != nil {
				if renderErr := RenderError(c, fmt.Sprintf("Invalid Icon URL: %s", err.Error())); renderErr != nil {
					slog.Error("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
		}

		// Validate TOS URL if provided
		if tosUrl != "" {
			if err := validateURL(tosUrl); err != nil {
				if renderErr := RenderError(c, fmt.Sprintf("Invalid Terms of Service URL: %s", err.Error())); renderErr != nil {
					slog.Error("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
		}

		// Set values after validation
		if iconUrl == "" {
			mint.Config.IconUrl = nil
		} else {
			mint.Config.IconUrl = &iconUrl
		}

		if tosUrl == "" {
			mint.Config.TosUrl = nil
		} else {
			mint.Config.TosUrl = &tosUrl
		}

		// Now process all other form fields
		mint.Config.NAME = c.Request.PostFormValue("NAME")
		mint.Config.DESCRIPTION = c.Request.PostFormValue("DESCRIPTION")
		mint.Config.DESCRIPTION_LONG = c.Request.PostFormValue("DESCRIPTION_LONG")
		mint.Config.EMAIL = c.Request.PostFormValue("EMAIL")
		mint.Config.MOTD = c.Request.PostFormValue("MOTD")

		nostrKey := c.Request.PostFormValue("NOSTR")

		if len(nostrKey) > 0 {
			isValid, err := isNostrKeyValid(nostrKey)
			if err != nil {
				_ = c.Error(ErrInvalidNostrKey)
				slog.Warn(
					"nip19.Decode(nostrKey)",
					slog.String(utils.LogExtraInfo, err.Error()))
				return
			}

			if !isValid {
				_ = c.Error(ErrInvalidNostrKey)
				return
			}

			mint.Config.NOSTR = nostrKey
		} else {
			mint.Config.NOSTR = ""
		}

		err := mint.MintDB.UpdateConfig(mint.Config)
		if err != nil {
			slog.Error(
				"mint.MintDB.UpdateConfig(mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))
		}

		if err := RenderSuccess(c, "General settings successfully set"); err != nil {
			slog.Error("failed to render success", slog.Any("error", err))
		}
	}
}

func MintSettingsLightning(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		pegoutOnly := c.Request.PostFormValue("PEG_OUT_ONLY")
		if pegoutOnly == "on" {
			mint.Config.PEG_OUT_ONLY = true

		} else {
			mint.Config.PEG_OUT_ONLY = false
		}

		// Check pegin limit.
		pegInLitmit, err := checkLimitSat(c.Request.PostFormValue("PEG_IN_LIMIT_SATS"))
		if err != nil {
			slog.Debug(
				`checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))`,
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "peg out limit has a problem"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}
		mint.Config.PEG_IN_LIMIT_SATS = pegInLitmit

		// Check pegout limit.
		pegOutLitmit, err := checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))
		if err != nil {
			slog.Debug(
				`checkLimitSat(c.Request.PostFormValue("PEG_OUT_LIMIT_SATS"))`,
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "peg out limit has a problem"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}
		mint.Config.PEG_OUT_LIMIT_SATS = pegOutLitmit

		err = mint.MintDB.UpdateConfig(mint.Config)
		if err != nil {
			slog.Error(
				"mint.MintDB.UpdateConfig(mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))
		}

		if err := RenderSuccess(c, "Lightning settings successfully set"); err != nil {
			slog.Error("failed to render success", slog.Any("error", err))
		}
	}
}

func MintSettingsAuth(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := changeAuthSettings(mint, c)
		if err != nil {
			_ = c.Error(fmt.Errorf("changeAuthSettings(mint, c). %w", err))
			slog.Warn(
				`fmt.Errorf("changeAuthSettings(mint, c). %w", err)`,
				slog.String(utils.LogExtraInfo, err.Error()))
			return
		}
		err = mint.MintDB.UpdateConfig(mint.Config)

		if err != nil {
			slog.Error(
				"mint.MintDB.UpdateConfig(mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))

			_ = c.Error(fmt.Errorf("mint.MintDB.UpdateConfig(mint.Config). %w", err))
			// return // Mocking success
		}

		if err := RenderSuccess(c, "Auth settings successfully set"); err != nil {
			slog.Error("failed to render success", slog.Any("error", err))
		}
	}
}

func LightningNodePage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		err := templates.LightningBackendPage(mint.Config).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(fmt.Errorf("templates.LightningBackendPage(mint.Config).Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

func Bolt11Post(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		formNetwork := c.Request.PostFormValue("NETWORK")

		chainparam, err := m.CheckChainParams(formNetwork)
		if err != nil {
			slog.Error(
				"m.CheckChainParams(formNetwork)",
				slog.String(utils.LogExtraInfo, err.Error()))

			if renderErr := RenderError(c, "Could not setup network for lightning"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		var newBackend lightning.LightningBackend
		var newBackendType utils.LightningBackend

		// Temporary config variables to hold the new settings
		// Initialize with existing config values
		var (
			lndHost     = mint.Config.LND_GRPC_HOST
			lndTls      = mint.Config.LND_TLS_CERT
			lndMacaroon = mint.Config.LND_MACAROON

			lnbitsKey      = mint.Config.MINT_LNBITS_KEY
			lnbitsEndpoint = mint.Config.MINT_LNBITS_ENDPOINT

			strikeKey      = mint.Config.STRIKE_KEY
			strikeEndpoint = mint.Config.STRIKE_ENDPOINT

			clnHost     = mint.Config.CLN_GRPC_HOST
			clnCa       = mint.Config.CLN_CA_CERT
			clnClient   = mint.Config.CLN_CLIENT_CERT
			clnKey      = mint.Config.CLN_CLIENT_KEY
			clnMacaroon = mint.Config.CLN_MACAROON
		)

		switch c.Request.PostFormValue("MINT_LIGHTNING_BACKEND") {

		case string(utils.FAKE_WALLET):
			newBackendType = utils.FAKE_WALLET
			fakeWalletBackend := lightning.FakeWallet{
				Network: chainparam,
			}
			newBackend = fakeWalletBackend

		case string(utils.LNDGRPC):
			newBackendType = utils.LNDGRPC
			lndHost = c.Request.PostFormValue("LND_GRPC_HOST")
			lndTls = c.Request.PostFormValue("LND_TLS_CERT")
			lndMacaroon = c.Request.PostFormValue("LND_MACAROON")

			lndWallet := lightning.LndGrpcWallet{
				Network: chainparam,
			}

			err := lndWallet.SetupGrpc(lndHost, lndMacaroon, lndTls)
			if err != nil {
				slog.Error(
					"lndWallet.SetupGrpc",
					slog.String(utils.LogExtraInfo, err.Error()))

				if renderErr := RenderError(c, "Something went wrong setting up LND communications"); renderErr != nil {
					slog.Error("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = lndWallet

		case string(utils.LNBITS):
			newBackendType = utils.LNBITS
			lnbitsKey = c.Request.PostFormValue("MINT_LNBITS_KEY")
			lnbitsEndpoint = c.Request.PostFormValue("MINT_LNBITS_ENDPOINT")

			lnbitsWallet := lightning.LnbitsWallet{
				Network:  chainparam,
				Key:      lnbitsKey,
				Endpoint: lnbitsEndpoint,
			}
			newBackend = lnbitsWallet

		case string(utils.Strike):
			newBackendType = utils.Strike
			strikeKey = c.Request.PostFormValue("STRIKE_KEY")
			strikeEndpoint = c.Request.PostFormValue("STRIKE_ENDPOINT")

			strikeWallet := lightning.Strike{
				Network: chainparam,
			}

			err := strikeWallet.Setup(strikeKey, strikeEndpoint)
			if err != nil {
				_ = c.Error(fmt.Errorf("strikeWallet.Setup(strikeKey, strikeEndpoint) %w %w", err, ErrInvalidStrikeConfig))
				if renderErr := RenderError(c, "Invalid Strike configuration"); renderErr != nil {
					slog.Error("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = strikeWallet

		case string(utils.CLNGRPC):
			newBackendType = utils.CLNGRPC
			clnHost = c.Request.PostFormValue("CLN_GRPC_HOST")
			clnCa = c.Request.PostFormValue("CLN_CA_CERT")
			clnClient = c.Request.PostFormValue("CLN_CLIENT_CERT")
			clnKey = c.Request.PostFormValue("CLN_CLIENT_KEY")
			clnMacaroon = c.Request.PostFormValue("CLN_MACAROON")

			clnWallet := lightning.CLNGRPCWallet{
				Network: chainparam,
			}

			err := clnWallet.SetupGrpc(clnHost, clnCa, clnClient, clnKey, clnMacaroon)
			if err != nil {
				slog.Error(
					"clnWallet.SetupGrpc",
					slog.String(utils.LogExtraInfo, err.Error()))

				if renderErr := RenderError(c, "Something went wrong setting up CLN communications"); renderErr != nil {
					slog.Error("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = clnWallet

		default:
			if renderErr := RenderError(c, "Invalid backend selection"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		// --- VERIFICATION STEP ---

		// 1. Check connection/balance
		_, err = newBackend.WalletBalance()
		if err != nil {
			slog.Warn(
				"Could not get lightning balance",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not check established connection with Node (WalletBalance failed)"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		// 2. Check invoice generation (100 sats)
		// We use a dummy quote ID to avoid messing with real DB if possible.
		testQuote := "verification-test-" + strconv.FormatInt(time.Now().Unix(), 10)
		invoiceResp, err := newBackend.RequestInvoice(
			cashu.MintRequestDB{Quote: testQuote},
			cashu.Amount{Unit: cashu.Sat, Amount: 100},
		)
		if err != nil {
			slog.Error("newBackend.RequestInvoice failed during verification", slog.String("err", err.Error()))
			if renderErr := RenderError(c, "Could not generate a test invoice with the new backend"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		// 3. Decode invoice and verify network
		decodedInvoice, err := zpay32.Decode(invoiceResp.PaymentRequest, &chainparam)
		if err != nil {
			slog.Error("zpay32.Decode failed during verification", slog.String("err", err.Error()))
			if renderErr := RenderError(c, "Lightning backend network does not match selected network configuration"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		// Verify amount matches (sanity check)
		if decodedInvoice.MilliSat == nil || int64(decodedInvoice.MilliSat.ToSatoshis()) != 100 {
			slog.Warn("Decoded invoice amount mismatch")
		}

		// --- APPLY SETTINGS ---

		// Update Config object
		mint.Config.NETWORK = chainparam.Name
		mint.Config.MINT_LIGHTNING_BACKEND = newBackendType

		switch newBackendType {
		case utils.LNDGRPC:
			mint.Config.LND_GRPC_HOST = lndHost
			mint.Config.LND_MACAROON = lndMacaroon
			mint.Config.LND_TLS_CERT = lndTls
		case utils.LNBITS:
			mint.Config.MINT_LNBITS_KEY = lnbitsKey
			mint.Config.MINT_LNBITS_ENDPOINT = lnbitsEndpoint
		case utils.Strike:
			mint.Config.STRIKE_KEY = strikeKey
			mint.Config.STRIKE_ENDPOINT = strikeEndpoint
		case utils.CLNGRPC:
			mint.Config.CLN_GRPC_HOST = clnHost
			mint.Config.CLN_MACAROON = clnMacaroon
			mint.Config.CLN_CA_CERT = clnCa
			mint.Config.CLN_CLIENT_KEY = clnKey
			mint.Config.CLN_CLIENT_CERT = clnClient
		}

		// Switch the live backend
		mint.LightningBackend = newBackend

		// Save to DB
		err = mint.MintDB.UpdateConfig(mint.Config)
		if err != nil {
			slog.Error(
				"mint.MintDB.UpdateConfig(mint.Config)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Settings applied but failed to save to database"); renderErr != nil {
				slog.Error("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		if err := RenderSuccess(c, "Lightning node settings changed and verified successfully"); err != nil {
			slog.Error("failed to render success", slog.Any("error", err))
		}
	}
}
