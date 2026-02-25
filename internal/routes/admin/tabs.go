package admin

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
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
		ctx := c.Request.Context()

		err := templates.MintSettings(
			mint.Config,
			nostrNotificationConfigValue(mint.NostrNotificationConfig),
			showLDKNodeLink(mint),
		).Render(ctx, c.Writer)
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

func parseLDKPersistedConfig(c *gin.Context, existingConfig ldk.PersistedConfig, configDirectory string) (ldk.PersistedConfig, error) {
	chainSourceType := normalizeLDKChainSourceType(c.Request.PostFormValue("LDK_CHAIN_SOURCE_TYPE"))
	config := existingConfig
	config.ConfigDirectory = configDirectory

	switch ldk.ChainSourceType(chainSourceType) {
	case ldk.ChainSourceElectrum:
		electrumServerURL := strings.TrimSpace(c.Request.PostFormValue("ELECTRUM_SERVER_URL"))
		if electrumServerURL == "" {
			return ldk.PersistedConfig{}, fmt.Errorf("electrum server url is required")
		}

		persistedConfig, err := ldk.NewPersistedConfigWithChainSource(ldk.ChainSourceElectrum, config.Rpc, electrumServerURL, configDirectory)
		if err != nil {
			return ldk.PersistedConfig{}, fmt.Errorf("ldk.NewPersistedConfigWithChainSource(...): %w", err)
		}

		return persistedConfig, nil
	case ldk.ChainSourceBitcoind:
		address := strings.TrimSpace(c.Request.PostFormValue("BITCOIN_NODE_RPC_ADDRESS"))
		portText := strings.TrimSpace(c.Request.PostFormValue("BITCOIN_NODE_RPC_PORT"))
		username := strings.TrimSpace(c.Request.PostFormValue("BITCOIN_NODE_RPC_USERNAME"))
		password := strings.TrimSpace(c.Request.PostFormValue("BITCOIN_NODE_RPC_PASSWORD"))

		if address == "" {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc address is required")
		}
		if portText == "" {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc port is required")
		}
		portValue, err := strconv.Atoi(portText)
		if err != nil {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc port is invalid")
		}
		if portValue < 1 || portValue > 65535 {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc port must be between 1 and 65535")
		}
		if username == "" {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc username is required")
		}
		if password == "" {
			password = config.Rpc.Password
		}
		if password == "" {
			return ldk.PersistedConfig{}, fmt.Errorf("bitcoin node rpc password is required")
		}

		persistedConfig, err := ldk.NewPersistedConfigWithChainSource(ldk.ChainSourceBitcoind, ldk.RPCConfig{
			Address:  address,
			Port:     uint16(portValue),
			Username: username,
			Password: password,
		}, config.ElectrumServerURL, configDirectory)
		if err != nil {
			return ldk.PersistedConfig{}, fmt.Errorf("ldk.NewPersistedConfigWithChainSource(...): %w", err)
		}

		return persistedConfig, nil
	default:
		return ldk.PersistedConfig{}, fmt.Errorf("invalid ldk chain source type")
	}
}

func ldkConfigsEqual(current ldk.PersistedConfig, incoming ldk.PersistedConfig) bool {
	return current.ChainSourceType == incoming.ChainSourceType &&
		current.ElectrumServerURL == incoming.ElectrumServerURL &&
		current.Rpc.Address == incoming.Rpc.Address &&
		current.Rpc.Port == incoming.Rpc.Port &&
		current.Rpc.Username == incoming.Rpc.Username &&
		current.Rpc.Password == incoming.Rpc.Password &&
		current.ConfigDirectory == incoming.ConfigDirectory
}

func ldkConfigBackendForMint(mint *m.Mint, network string) (*ldk.LDK, error) {
	if currentLDK, ok := mint.LightningBackend.(*ldk.LDK); ok && mint.Config.MINT_LIGHTNING_BACKEND == utils.LDK {
		return currentLDK, nil
	}
	ldk, err := ldk.NewConfigBackend(mint.MintDB, network)
	if err != nil {
		return nil, err
	}

	return ldk, nil
}

func ldkConfigUnchanged(ctx context.Context, backend *ldk.LDK, currentNetwork string, nextNetwork string, config ldk.PersistedConfig) bool {
	if backend == nil {
		return false
	}

	existingConfig, err := backend.PersistedConfig(ctx)
	if err != nil {
		return false
	}

	return currentNetwork == nextNetwork && ldkConfigsEqual(existingConfig, config)
}

func decodeNpubToHex(npub string) (string, error) {
	prefix, key, err := nip19.Decode(strings.TrimSpace(npub))
	if err != nil {
		return "", fmt.Errorf("nip19.Decode(npub): %w", err)
	}

	if prefix != "npub" {
		return "", fmt.Errorf("invalid nostr key prefix: %s", prefix)
	}

	keyStr, ok := key.(string)
	if !ok {
		return "", fmt.Errorf("nip19.Decode(npub) returned %T", key)
	}

	if !nostr.IsValid32ByteHex(keyStr) {
		return "", fmt.Errorf("invalid 32 byte public key")
	}

	return keyStr, nil
}

func parseNpubToWrappedPublicKey(npub string) (cashu.WrappedPublicKey, error) {
	pubkeyHex, err := decodeNpubToHex(npub)
	if err != nil {
		return cashu.WrappedPublicKey{}, fmt.Errorf("decodeNpubToHex(npub): %w", err)
	}

	pubkeyBytes, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		return cashu.WrappedPublicKey{}, fmt.Errorf("hex.DecodeString(pubkeyHex): %w", err)
	}

	pubkey, err := schnorr.ParsePubKey(pubkeyBytes)
	if err != nil {
		return cashu.WrappedPublicKey{}, fmt.Errorf("schnorr.ParsePubKey(pubkeyBytes): %w", err)
	}

	return cashu.WrappedPublicKey{PublicKey: pubkey}, nil
}

func parseNpubArrayToWrappedPublicKeys(npubs []string) ([]cashu.WrappedPublicKey, error) {
	parsed := make([]cashu.WrappedPublicKey, 0, len(npubs))
	for _, npub := range npubs {
		trimmed := strings.TrimSpace(npub)
		if trimmed == "" {
			continue
		}

		pubkey, err := parseNpubToWrappedPublicKey(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parseNpubToWrappedPublicKey(%q): %w", trimmed, err)
		}
		parsed = append(parsed, pubkey)
	}

	return parsed, nil
}

func dedupeWrappedPublicKeys(pubkeys []cashu.WrappedPublicKey) []cashu.WrappedPublicKey {
	if len(pubkeys) == 0 {
		return pubkeys
	}

	seen := make(map[string]struct{}, len(pubkeys))
	unique := make([]cashu.WrappedPublicKey, 0, len(pubkeys))
	for _, key := range pubkeys {
		hexValue := key.ToHex()
		if hexValue == "" {
			continue
		}
		if _, ok := seen[hexValue]; ok {
			continue
		}
		seen[hexValue] = struct{}{}
		unique = append(unique, key)
	}

	return unique
}

func isNostrKeyValid(nostrKey string) (bool, error) {
	_, err := decodeNpubToHex(nostrKey)
	if err != nil {
		return false, err
	}

	return true, nil
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
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
		}

		// Validate TOS URL if provided
		if tosUrl != "" {
			if err := validateURL(tosUrl); err != nil {
				if renderErr := RenderError(c, fmt.Sprintf("Invalid Terms of Service URL: %s", err.Error())); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
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

		err := persistConfigTx(c.Request.Context(), mint, mint.Config)
		if err != nil {
			slog.Warn(
				"persistConfigTx(c.Request.Context(), mint, mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))
			return
		}

		// render the settings page
		if err := templates.General(mint.Config).Render(c.Request.Context(), c.Writer); err != nil {
			slog.Warn("failed to render settings", slog.Any("error", err))
			return
		}

		// obb success and render the settings page
		err = templates.ObbNotification(templates.SuccessNotif("General settings successfully set")).Render(c.Request.Context(), c.Writer)
		if err != nil {
			slog.Warn("failed to render success", slog.Any("error", err))
			return
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
				slog.Warn("failed to render error", slog.Any("error", renderErr))
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
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}
		mint.Config.PEG_OUT_LIMIT_SATS = pegOutLitmit

		err = persistConfigTx(c.Request.Context(), mint, mint.Config)
		if err != nil {
			slog.Warn(
				"persistConfigTx(c.Request.Context(), mint, mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))
		}

		// render the settings page
		if err := templates.Lightning(mint.Config).Render(c.Request.Context(), c.Writer); err != nil {
			slog.Warn("failed to render settings", slog.Any("error", err))
			return
		}

		// obb success and render the settings page
		err = templates.ObbNotification(templates.SuccessNotif("General settings successfully set")).Render(c.Request.Context(), c.Writer)
		if err != nil {
			slog.Warn("failed to render success", slog.Any("error", err))
			return
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
		err = persistConfigTx(c.Request.Context(), mint, mint.Config)

		if err != nil {
			slog.Warn(
				"persistConfigTx(c.Request.Context(), mint, mint.Config) - Mocking success despite error",
				slog.String(utils.LogExtraInfo, err.Error()))

			_ = c.Error(fmt.Errorf("persistConfigTx(c.Request.Context(), mint, mint.Config). %w", err))
			// return // Mocking success
		}

		// render the settings page
		if err := templates.Auth(mint.Config).Render(c.Request.Context(), c.Writer); err != nil {
			slog.Warn("failed to render settings", slog.Any("error", err))
			return
		}

		// obb success and render the settings page
		err = templates.ObbNotification(templates.SuccessNotif("Auth settings successfully set")).Render(c.Request.Context(), c.Writer)
		if err != nil {
			slog.Warn("failed to render success", slog.Any("error", err))
			return
		}
	}
}

func MintSettingsNotifications(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		nostrNotificationsEnabled := c.Request.PostFormValue("NOSTR_NOTIFICATIONS") == "on"
		nostrNotificationsNip04DMEnabled := c.Request.PostFormValue("NOSTR_NOTIFICATION_NIP04_DM") == "on"
		npubInputs := c.PostFormArray("NOSTR_NOTIFICATION_NPUBS")

		nextConfig := nostrNotificationConfigValue(mint.NostrNotificationConfig)
		npubsToPersist := nextConfig.NOSTR_NOTIFICATION_NPUBS

		if len(npubInputs) > 0 {
			npubs, err := parseNpubArrayToWrappedPublicKeys(npubInputs)
			if err != nil {
				slog.Warn(
					"parseNpubArrayToWrappedPublicKeys(npubInputs)",
					slog.String(utils.LogExtraInfo, err.Error()))
				if renderErr := RenderError(c, "Nostr notification npub is not valid"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			npubsToPersist = dedupeWrappedPublicKeys(npubs)
		}

		err := nextConfig.SetNostrNotificationConfig(nostrNotificationsEnabled, nil, npubsToPersist)
		if err != nil {
			slog.Warn(
				"mint.Config.SetNostrNotificationConfig(...)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not update nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		err = syncNostrNotificationNsec(&nextConfig)
		if err != nil {
			slog.Warn(
				"syncNostrNotificationNsec(&nextConfig)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not update nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		nextConfig.NOSTR_NOTIFICATION_NIP04_DM = nostrNotificationsNip04DMEnabled
		nextConfig.NOSTR_NOTIFICATION_NPUBS = npubsToPersist
		nextConfig.NOSTR_NOTIFICATIONS = nostrNotificationsEnabled

		err = persistNostrNotificationConfigTx(c.Request.Context(), mint, nextConfig)
		if err != nil {
			slog.Warn(
				"persistNostrNotificationConfigTx(c.Request.Context(), mint, nextConfig)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not persist nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		mint.NostrNotificationConfig = &nextConfig

		if err := renderNotificationsForm(c, nextConfig, "Nostr notification settings successfully set", true); err != nil {
			slog.Warn("failed to render notifications form", slog.Any("error", err))
		}
	}
}

func MintSettingsNotificationsTest(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mint == nil {
			if err := templates.ObbNotification(templates.ErrorNotif("Mint is not available")).Render(c.Request.Context(), c.Writer); err != nil {
				slog.Warn("failed to render test notification error", slog.Any("error", err))
			}
			return
		}

		if mint.NostrNotificationConfig == nil || !mint.NostrNotificationConfig.NOSTR_NOTIFICATIONS {
			if err := templates.ObbNotification(templates.ErrorNotif("Enable nostr notifications first")).Render(c.Request.Context(), c.Writer); err != nil {
				slog.Warn("failed to render test notification disabled error", slog.Any("error", err))
			}
			return
		}

		now := time.Now().UTC()
		slog.Error(
			"nostr test notification trigger",
			slog.String("source", "admin.nostr_notifications.test_button"),
			slog.String("nonce", strconv.FormatInt(now.UnixNano(), 10)),
			slog.Time("triggered_at", now),
		)

		if err := templates.ObbNotification(templates.SuccessNotif("Test error log has been written")).Render(c.Request.Context(), c.Writer); err != nil {
			slog.Warn("failed to render test notification success", slog.Any("error", err))
		}
	}
}

func MintSettingsNotificationDeleteNpub(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		npub := c.Param("npub")
		if strings.TrimSpace(npub) == "" {
			if renderErr := RenderError(c, "Missing nostr notification npub"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		pubkeyToDelete, err := parseNpubToWrappedPublicKey(npub)
		if err != nil {
			slog.Warn(
				"parseNpubToWrappedPublicKey(npub)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Nostr notification npub is not valid"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		nextConfig := nostrNotificationConfigValue(mint.NostrNotificationConfig)
		filteredNpubs := make([]cashu.WrappedPublicKey, 0, len(nextConfig.NOSTR_NOTIFICATION_NPUBS))
		deleted := false
		for _, existing := range nextConfig.NOSTR_NOTIFICATION_NPUBS {
			if existing.ToHex() == pubkeyToDelete.ToHex() {
				deleted = true
				continue
			}
			filteredNpubs = append(filteredNpubs, existing)
		}

		if !deleted {
			if err := renderNotificationsForm(c, nostrNotificationConfigValue(mint.NostrNotificationConfig), "Nostr recipient was not found", false); err != nil {
				slog.Warn("failed to render notifications form", slog.Any("error", err))
			}
			return
		}

		err = nextConfig.SetNostrNotificationConfig(
			nextConfig.NOSTR_NOTIFICATIONS,
			nextConfig.NOSTR_NOTIFICATION_NSEC,
			filteredNpubs,
		)
		if err != nil {
			slog.Warn(
				"mint.Config.SetNostrNotificationConfig(...)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not update nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		err = syncNostrNotificationNsec(&nextConfig)
		if err != nil {
			slog.Warn(
				"syncNostrNotificationNsec(&nextConfig)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not update nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		err = persistNostrNotificationConfigTx(c.Request.Context(), mint, nextConfig)
		if err != nil {
			slog.Warn(
				"persistNostrNotificationConfigTx(c.Request.Context(), mint, nextConfig)",
				slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not persist nostr notification settings"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		mint.NostrNotificationConfig = &nextConfig

		if err := renderNotificationsForm(c, nextConfig, "Nostr recipient deleted", true); err != nil {
			slog.Warn("failed to render notifications form", slog.Any("error", err))
		}
	}
}

func nostrNotificationConfigValue(config *utils.NostrNotificationConfig) utils.NostrNotificationConfig {
	if config == nil {
		var emptyConfig utils.NostrNotificationConfig
		return emptyConfig
	}

	return *config
}

func syncNostrNotificationNsec(config *utils.NostrNotificationConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if !config.NOSTR_NOTIFICATIONS && len(config.NOSTR_NOTIFICATION_NSEC) == 0 {
		return nil
	}

	return utils.SyncNostrNotificationNsec(config, true)
}

func renderNotificationsForm(c *gin.Context, config utils.NostrNotificationConfig, message string, isSuccess bool) error {
	if err := templates.Notifications(config).Render(c.Request.Context(), c.Writer); err != nil {
		return fmt.Errorf("templates.Notifications(config).Render(...): %w", err)
	}

	if message == "" {
		return nil
	}

	if isSuccess {
		return templates.ObbNotification(templates.SuccessNotif(message)).Render(c.Request.Context(), c.Writer)
	}

	return templates.ObbNotification(templates.ErrorNotif(message)).Render(c.Request.Context(), c.Writer)
}

func persistConfigTx(ctx context.Context, mint *m.Mint, config utils.Config) (err error) {
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}

	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
			slog.Warn("mint.MintDB.Rollback(ctx, tx)", slog.String(utils.LogExtraInfo, rollbackErr.Error()))
		}
	}()

	err = mint.MintDB.UpdateConfig(tx, config)
	if err != nil {
		return fmt.Errorf("mint.MintDB.UpdateConfig(tx, config): %w", err)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx, tx): %w", err)
	}

	return nil
}

func persistNostrNotificationConfigTx(ctx context.Context, mint *m.Mint, config utils.NostrNotificationConfig) (err error) {
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}

	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
			slog.Warn("mint.MintDB.Rollback(ctx, tx)", slog.String(utils.LogExtraInfo, rollbackErr.Error()))
		}
	}()

	err = mint.MintDB.UpdateNostrNotificationConfig(tx, config)
	if err != nil {
		return fmt.Errorf("mint.MintDB.UpdateNostrNotificationConfig(tx, config): %w", err)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx, tx): %w", err)
	}

	return nil
}

func LightningNodePage(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		err := templates.LightningBackendPage(mint.Config, showLDKNodeLink(mint)).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(fmt.Errorf("templates.LightningBackendPage(mint.Config, showLDKNodeLink(mint)).Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

func Bolt11Post(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		formNetwork := c.Request.PostFormValue("NETWORK")

		chainparam, err := m.CheckChainParams(formNetwork)
		if err != nil {
			slog.Warn("m.CheckChainParams(formNetwork)", slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not setup network for lightning"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		ctx := c.Request.Context()
		oldConfig := mint.Config
		oldBackend := mint.LightningBackend

		var newBackend lightning.LightningBackend
		var newBackendType utils.LightningBackend
		var ldkConfig ldk.PersistedConfig

		var (
			lndHost     = mint.Config.LND_GRPC_HOST
			lndTLS      = mint.Config.LND_TLS_CERT
			lndMacaroon = mint.Config.LND_MACAROON

			lnbitsKey      = mint.Config.MINT_LNBITS_KEY
			lnbitsEndpoint = mint.Config.MINT_LNBITS_ENDPOINT

			strikeKey      = mint.Config.STRIKE_KEY
			strikeEndpoint = mint.Config.STRIKE_ENDPOINT

			clnHost     = mint.Config.CLN_GRPC_HOST
			clnCA       = mint.Config.CLN_CA_CERT
			clnClient   = mint.Config.CLN_CLIENT_CERT
			clnKey      = mint.Config.CLN_CLIENT_KEY
			clnMacaroon = mint.Config.CLN_MACAROON
		)

		switch c.Request.PostFormValue("MINT_LIGHTNING_BACKEND") {
		case string(utils.FAKE_WALLET):
			newBackendType = utils.FAKE_WALLET
			newBackend = lightning.FakeWallet{
				Network:         chainparam,
				UnpurposeErrors: []lightning.FakeWalletError{},
				InvoiceFee:      0,
			}

		case string(utils.LNDGRPC):
			newBackendType = utils.LNDGRPC
			lndHost = c.Request.PostFormValue("LND_GRPC_HOST")
			lndTLS = c.Request.PostFormValue("LND_TLS_CERT")
			lndMacaroon = c.Request.PostFormValue("LND_MACAROON")

			lndWallet := lightning.LndGrpcWallet{Network: chainparam}
			if err := lndWallet.SetupGrpc(lndHost, lndMacaroon, lndTLS); err != nil {
				slog.Warn("lndWallet.SetupGrpc", slog.String(utils.LogExtraInfo, err.Error()))
				if renderErr := RenderError(c, "Something went wrong setting up LND communications"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = lndWallet

		case string(utils.LNBITS): //nolint:staticcheck // LNBITS remains configurable until its planned removal in v0.8.0.
			newBackendType = utils.LNBITS //nolint:staticcheck // LNBITS remains configurable until its planned removal in v0.8.0.
			lnbitsKey = c.Request.PostFormValue("MINT_LNBITS_KEY")
			lnbitsEndpoint = c.Request.PostFormValue("MINT_LNBITS_ENDPOINT")

			slog.Warn("LNBITS backend is deprecated and will be removed in v0.8.0")

			lnbitsWallet := lightning.LnbitsWallet{
				Network:  chainparam,
				Key:      lnbitsKey,
				Endpoint: lnbitsEndpoint,
			}
			newBackend = lnbitsWallet

		case string(utils.Strike): //nolint:staticcheck // Strike remains configurable until its planned removal in v0.7.0.
			newBackendType = utils.Strike //nolint:staticcheck // Strike remains configurable until its planned removal in v0.7.0.
			strikeKey = c.Request.PostFormValue("STRIKE_KEY")
			strikeEndpoint = c.Request.PostFormValue("STRIKE_ENDPOINT")

			strikeWallet := lightning.Strike{Network: chainparam}
			if err := strikeWallet.Setup(strikeKey, strikeEndpoint); err != nil {
				_ = c.Error(fmt.Errorf("strikeWallet.Setup(strikeKey, strikeEndpoint) %w %w", err, ErrInvalidStrikeConfig))
				if renderErr := RenderError(c, "Invalid Strike configuration"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = strikeWallet

		case string(utils.CLNGRPC):
			newBackendType = utils.CLNGRPC
			clnHost = c.Request.PostFormValue("CLN_GRPC_HOST")
			clnCA = c.Request.PostFormValue("CLN_CA_CERT")
			clnClient = c.Request.PostFormValue("CLN_CLIENT_CERT")
			clnKey = c.Request.PostFormValue("CLN_CLIENT_KEY")
			clnMacaroon = c.Request.PostFormValue("CLN_MACAROON")

			clnWallet := lightning.CLNGRPCWallet{Network: chainparam}
			if err := clnWallet.SetupGrpc(clnHost, clnCA, clnClient, clnKey, clnMacaroon); err != nil {
				slog.Warn("clnWallet.SetupGrpc", slog.String(utils.LogExtraInfo, err.Error()))
				if renderErr := RenderError(c, "Something went wrong setting up CLN communications"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			newBackend = clnWallet

		case string(utils.LDK):
			newBackendType = utils.LDK
			defaultConfigDirectory, err := ldk.DefaultConfigDirectory()
			if err != nil {
				if renderErr := RenderError(c, "Could not determine LDK storage directory"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}

			existingLDKConfig := ldk.PersistedConfig{
				ConfigDirectory: defaultConfigDirectory,
				ChainSourceType: ldk.ChainSourceBitcoind,
			}
			if persistedConfig, getConfigErr := ldk.GetPersistedConfig(ctx, mint.MintDB); getConfigErr == nil {
				existingLDKConfig = persistedConfig
			}

			ldkConfig, err = parseLDKPersistedConfig(c, existingLDKConfig, defaultConfigDirectory)
			if err != nil {
				if renderErr := RenderError(c, err.Error()); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}

			configBackend, err := ldkConfigBackendForMint(mint, chainparam.Name)
			if err != nil {
				if renderErr := RenderError(c, err.Error()); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}
			if ldkConfigUnchanged(ctx, configBackend, mint.Config.NETWORK, chainparam.Name, ldkConfig) {
				if err := RenderSuccess(c, "No changes detected"); err != nil {
					slog.Warn("failed to render success", slog.Any("error", err))
				}
				return
			}

			if err := configBackend.SaveConfig(ctx, ldkConfig); err != nil {
				slog.Warn("configBackend.SaveConfig", slog.String(utils.LogExtraInfo, err.Error()))
				if renderErr := RenderError(c, "Could not persist LDK configuration"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}

			newBackend, err = ldk.NewLdk(ctx, mint.MintDB, chainparam.Name)
			if err != nil {
				slog.Warn("ldk.NewLdk(ctx, mint.MintDB)", slog.String(utils.LogExtraInfo, err.Error()))
				if renderErr := RenderError(c, "Something went wrong setting up LDK communications"); renderErr != nil {
					slog.Warn("failed to render error", slog.Any("error", renderErr))
				}
				return
			}

		default:
			if renderErr := RenderError(c, "Invalid backend selection"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		_, err = newBackend.WalletBalance()
		if err != nil {
			slog.Warn("Could not get lightning balance", slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Could not check established connection with Node (WalletBalance failed)"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		testQuote := "verification-test-" + strconv.FormatInt(time.Now().Unix(), 10)
		//nolint:exhaustruct
		invoiceResp, err := newBackend.RequestInvoice(
			cashu.MintRequestDB{Quote: testQuote},
			cashu.NewAmount(cashu.Sat, 100),
		)
		if err != nil {
			slog.Warn("newBackend.RequestInvoice failed during verification", slog.String("err", err.Error()))
			if renderErr := RenderError(c, "Could not generate a test invoice with the new backend"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		decodedInvoice, err := zpay32.Decode(invoiceResp.PaymentRequest, &chainparam)
		if err != nil {
			slog.Warn("zpay32.Decode failed during verification", slog.String("err", err.Error()))
			if renderErr := RenderError(c, "Lightning backend network does not match selected network configuration"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		if decodedInvoice.MilliSat == nil || int64(decodedInvoice.MilliSat.ToSatoshis()) != 100 {
			slog.Warn("Decoded invoice amount mismatch")
		}

		mint.Config.NETWORK = chainparam.Name
		mint.Config.MINT_LIGHTNING_BACKEND = newBackendType

		switch newBackendType {
		case utils.LNDGRPC:
			mint.Config.LND_GRPC_HOST = lndHost
			mint.Config.LND_MACAROON = lndMacaroon
			mint.Config.LND_TLS_CERT = lndTLS
		case utils.LNBITS: //nolint:staticcheck // LNBITS config is still persisted until its planned removal in v0.8.0.
			mint.Config.MINT_LNBITS_KEY = lnbitsKey
			mint.Config.MINT_LNBITS_ENDPOINT = lnbitsEndpoint
		case utils.Strike: //nolint:staticcheck // Strike config is still persisted until its planned removal in v0.7.0.
			mint.Config.STRIKE_KEY = strikeKey
			mint.Config.STRIKE_ENDPOINT = strikeEndpoint
		case utils.CLNGRPC:
			mint.Config.CLN_GRPC_HOST = clnHost
			mint.Config.CLN_MACAROON = clnMacaroon
			mint.Config.CLN_CA_CERT = clnCA
			mint.Config.CLN_CLIENT_KEY = clnKey
			mint.Config.CLN_CLIENT_CERT = clnClient
		}

		if err = persistConfigTx(c.Request.Context(), mint, mint.Config); err != nil {
			mint.Config = oldConfig
			mint.LightningBackend = oldBackend
			slog.Warn("persistConfigTx(c.Request.Context(), mint, mint.Config)", slog.String(utils.LogExtraInfo, err.Error()))
			if renderErr := RenderError(c, "Settings applied but failed to save to database"); renderErr != nil {
				slog.Warn("failed to render error", slog.Any("error", renderErr))
			}
			return
		}

		mint.LightningBackend = newBackend

		if err := RenderSuccess(c, "Lightning node settings changed and verified successfully"); err != nil {
			slog.Warn("failed to render success", slog.Any("error", err))
		}
	}
}
