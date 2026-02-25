package admin

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

var ldkBackendGetter = getLDKBackend

var ldkPeerSummariesLoader = func(backend *ldk.LDK) ([]ldk.LDKPeerSummary, error) {
	return backend.PeerSummaries()
}

var ldkChannelSummariesLoader = func(backend *ldk.LDK) ([]ldk.LDKChannelSummary, error) {
	return backend.ChannelSummaries()
}

func LdkNodePage(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := renderLdkPage(c, m, templates.LdkSectionOnchain)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func LdkLightningPage(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := renderLdkPage(c, m, templates.LdkSectionLightning)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func LdkPaymentsPage(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := renderLdkPage(c, m, templates.LdkSectionPayments)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func renderLdkPage(c *gin.Context, m *mint.Mint, section templates.LdkSection) error {
	return templates.LdkPageShell(showLDKNodeLink(m), section, ldkPageContent(section)).Render(c.Request.Context(), c.Writer)
}

func ldkPageContent(section templates.LdkSection) templ.Component {
	switch section {
	case templates.LdkSectionLightning:
		return templates.LdkLightningPageContent()
	case templates.LdkSectionPayments:
		return templates.LdkPaymentsPageContent()
	default:
		return templates.LdkOnchainPageContent()
	}
}

func LdkBalancesFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			slog.Error("ldk backend type assertion failed",
				slog.String("event", "ldk_backend_type_assert_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderErr := templates.LdkBalancesErrorFragment("Could not load LDK balances").Render(c.Request.Context(), c.Writer)
			if renderErr != nil {
				_ = c.Error(renderErr)
			}
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances",
				slog.String("event", "ldk_balance_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderErr := templates.LdkBalancesErrorFragment("Could not load LDK balances").Render(c.Request.Context(), c.Writer)
			if renderErr != nil {
				_ = c.Error(renderErr)
			}
			return
		}

		totalOnchainBalance, availableOnchainBalance := formatLdkOnchainBalances(balances)

		c.Status(200)
		err = templates.LdkBalancesFragment(totalOnchainBalance, availableOnchainBalance).Render(c.Request.Context(), c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func LdkChannelsFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			slog.Error("ldk backend type assertion failed",
				slog.String("event", "ldk_backend_type_assert_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Status(200)
			if renderErr := templates.LdkChannelsErrorFragment("Could not load channels").Render(c.Request.Context(), c.Writer); renderErr != nil {
				_ = c.Error(renderErr)
			}
			return
		}

		channels, err := ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch ldk channels",
				slog.String("event", "ldk_channels_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Status(200)
			if renderErr := templates.LdkChannelsErrorFragment("Could not load channels").Render(c.Request.Context(), c.Writer); renderErr != nil {
				_ = c.Error(renderErr)
			}
			return
		}

		rows := mapLdkChannelRows(channels)

		c.Status(200)
		if renderErr := templates.LdkChannelsFragment(rows).Render(c.Request.Context(), c.Writer); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkNetworkSummaryFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := ldkBackendGetter(m)
		if err != nil {
			slog.Error("ldk backend type assertion failed",
				slog.String("event", "ldk_backend_type_assert_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNetworkSummaryError(c)
			return
		}

		peers, err := ldkPeerSummariesLoader(ldkBackend)
		if err != nil {
			slog.Error("could not fetch ldk peers",
				slog.String("event", "ldk_peers_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNetworkSummaryError(c)
			return
		}

		channels, err := ldkChannelSummariesLoader(ldkBackend)
		if err != nil {
			slog.Error("could not fetch ldk channels for summary",
				slog.String("event", "ldk_channels_summary_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNetworkSummaryError(c)
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances for lightning summary",
				slog.String("event", "ldk_lightning_summary_balance_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNetworkSummaryError(c)
			return
		}

		summary := mapLdkNetworkSummary(peers, channels)
		lightningBalance := formatLdkLightningBalance(balances)

		c.Status(200)
		if renderErr := templates.LdkNetworkSummaryFragment(lightningBalance, summary.TotalPeers, summary.ActivePeers, summary.TotalChannels, summary.ActiveChannels).Render(c.Request.Context(), c.Writer); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkAddressFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkActionPanelError(c, "Could not generate on-chain address")
			return
		}

		address, err := ldkBackend.NewOnchainAddress()
		if err != nil {
			slog.Error("could not generate ldk on-chain address",
				slog.String("event", "ldk_new_onchain_address_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkActionPanelError(c, "Could not generate on-chain address")
			return
		}

		qrCode, err := generateQR(address)
		if err != nil {
			slog.Error("could not generate on-chain address qr",
				slog.String("event", "ldk_new_onchain_address_qr_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkActionPanelError(c, "Could not generate on-chain address")
			return
		}

		c.Status(200)
		if renderErr := templates.LdkAddressFragment(address, qrCode).Render(c.Request.Context(), c.Writer); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkOpenChannelFormFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkActionPanelError(c, "Could not load channel form")
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances for channel form",
				slog.String("event", "ldk_channel_form_balance_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkActionPanelError(c, "Could not load channel form")
			return
		}

		maxSats := maxChannelSatsFromOnchain(balances.AvailableOnchainSats)
		c.Status(200)
		if renderErr := templates.LdkOpenChannelFormFragment(maxSats).Render(c.Request.Context(), c.Writer); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkOpenChannel(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		peerEndpoint := c.PostForm("peer_endpoint")
		pubkey, address, err := parseLdkPeerEndpoint(peerEndpoint)
		if err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}

		amountText := strings.TrimSpace(c.PostForm("sats_amount"))
		satsAmount, err := strconv.ParseUint(amountText, 10, 64)
		if err != nil {
			renderLdkNoSwapError(c, "Sats amount must be a positive integer")
			return
		}

		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkNoSwapError(c, "Could not open channel")
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances for opening channel",
				slog.String("event", "ldk_open_channel_balance_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, "Could not open channel")
			return
		}

		maxSats := maxChannelSatsFromOnchain(balances.AvailableOnchainSats)
		if err := validateChannelAmount(satsAmount, maxSats); err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}

		err = ldkBackend.OpenChannel(pubkey, address, satsAmount)
		if err != nil {
			slog.Error("ldk open channel failed",
				slog.String("event", "ldk_open_channel_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, mapOpenChannelError(err))
			return
		}

		channels, err := ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch channels after open channel",
				slog.String("event", "ldk_channels_fetch_after_open_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, "Channel opening started, but channels could not be refreshed")
			return
		}

		rows := mapLdkChannelRows(channels)
		balances, err = ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch balances after open channel",
				slog.String("event", "ldk_balance_fetch_after_open_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
		}
		networkSummary, networkErr := fetchLdkNetworkSummary(ldkBackend, channels)

		c.Status(200)
		if renderErr := writeLdkMutationSuccessPayload(c.Request.Context(), c.Writer, rows, balances, err, networkSummary, networkErr, "Channel opening started"); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkCloseChannel(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID := strings.TrimSpace(c.PostForm("channel_id"))
		counterpartyPub := strings.TrimSpace(c.PostForm("counterparty_pub"))

		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkNoSwapError(c, "Unable to start cooperative close")
			return
		}

		channels, err := ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch channels before close channel",
				slog.String("event", "ldk_channels_fetch_before_close_failed"),
				slog.String("action", "close"),
				slog.String("channel_id", channelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, "Unable to start cooperative close")
			return
		}

		channel, err := findLdkChannelForAction(channels, channelID, counterpartyPub)
		if err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}
		if err := validateCooperativeClose(channel); err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}

		if err := ldkBackend.CloseChannel(channel.ChannelID, channel.CounterpartyPub); err != nil {
			slog.Error("ldk close channel failed",
				slog.String("event", "ldk_close_channel_failed"),
				slog.String("action", "close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, mapCloseChannelError(err, false))
			return
		}

		channels, err = ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch channels after close channel",
				slog.String("event", "ldk_channels_fetch_after_close_failed"),
				slog.String("action", "close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapSuccess(c, "Cooperative close started, but the channel list could not be refreshed")
			return
		}

		rows := mapLdkChannelRows(channels)
		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch balances after close channel",
				slog.String("event", "ldk_balance_fetch_after_close_failed"),
				slog.String("action", "close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
		}
		networkSummary, networkErr := fetchLdkNetworkSummary(ldkBackend, channels)

		c.Status(200)
		if renderErr := writeLdkMutationSuccessPayload(c.Request.Context(), c.Writer, rows, balances, err, networkSummary, networkErr, "Cooperative close started"); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkForceCloseChannel(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID := strings.TrimSpace(c.PostForm("channel_id"))
		counterpartyPub := strings.TrimSpace(c.PostForm("counterparty_pub"))

		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkNoSwapError(c, "Unable to start force close")
			return
		}

		channels, err := ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch channels before force close channel",
				slog.String("event", "ldk_channels_fetch_before_force_close_failed"),
				slog.String("action", "force_close"),
				slog.String("channel_id", channelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, "Unable to start force close")
			return
		}

		channel, err := findLdkChannelForAction(channels, channelID, counterpartyPub)
		if err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}
		if err := validateForceClose(channel); err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}

		if err := ldkBackend.ForceCloseChannel(channel.ChannelID, channel.CounterpartyPub); err != nil {
			slog.Error("ldk force close channel failed",
				slog.String("event", "ldk_force_close_channel_failed"),
				slog.String("action", "force_close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, mapCloseChannelError(err, true))
			return
		}

		channels, err = ldkBackend.ChannelSummaries()
		if err != nil {
			slog.Error("could not fetch channels after force close channel",
				slog.String("event", "ldk_channels_fetch_after_force_close_failed"),
				slog.String("action", "force_close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapSuccess(c, "Force close started, but the channel list could not be refreshed")
			return
		}

		rows := mapLdkChannelRows(channels)
		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch balances after force close channel",
				slog.String("event", "ldk_balance_fetch_after_force_close_failed"),
				slog.String("action", "force_close"),
				slog.String("channel_id", channel.ChannelID),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
		}
		networkSummary, networkErr := fetchLdkNetworkSummary(ldkBackend, channels)

		c.Status(200)
		if renderErr := writeLdkMutationSuccessPayload(c.Request.Context(), c.Writer, rows, balances, err, networkSummary, networkErr, "Force close started for the offline channel"); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func parseLdkPeerEndpoint(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", fmt.Errorf("peer endpoint is required")
	}

	parts := strings.SplitN(value, "@", 3)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("peer endpoint must be in the format pubkey@address")
	}
	if strings.Contains(parts[1], "@") {
		return "", "", fmt.Errorf("peer endpoint must contain only one @ separator")
	}

	pubkeyHex := strings.TrimSpace(parts[0])
	if pubkeyHex == "" {
		return "", "", fmt.Errorf("peer public key is required before @")
	}
	if len(pubkeyHex) != 66 {
		return "", "", fmt.Errorf("peer public key must be a 33-byte compressed key")
	}

	pubkeyBytes, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		return "", "", fmt.Errorf("peer public key must be valid hex")
	}

	_, err = btcec.ParsePubKey(pubkeyBytes)
	if err != nil {
		return "", "", fmt.Errorf("peer public key is invalid")
	}

	address := strings.TrimSpace(parts[1])
	if address == "" {
		return "", "", fmt.Errorf("peer address is required after @")
	}
	if strings.ContainsAny(address, "\n\r\t") {
		return "", "", fmt.Errorf("peer address contains invalid whitespace")
	}

	return pubkeyHex, address, nil
}

func maxChannelSatsFromOnchain(onchain uint64) uint64 {
	return onchain * 95 / 100
}

func validateChannelAmount(amount uint64, maxSats uint64) error {
	if maxSats == 0 {
		return fmt.Errorf("on-chain balance is too low to open a channel")
	}
	if amount == 0 {
		return fmt.Errorf("sats amount must be greater than 0")
	}
	if amount > maxSats {
		return fmt.Errorf("sats amount exceeds max allowed (%d sats, 95%% of on-chain balance)", maxSats)
	}
	return nil
}

func mapOpenChannelError(err error) string {
	lower := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lower, "insufficient"), strings.Contains(lower, "balance"), strings.Contains(lower, "fund"):
		return "Insufficient on-chain balance to open channel"
	case strings.Contains(lower, "connect"), strings.Contains(lower, "socket"), strings.Contains(lower, "address"), strings.Contains(lower, "dns"):
		return "Could not connect to peer address"
	case strings.Contains(lower, "public key"), strings.Contains(lower, "pubkey"):
		return "Peer public key is invalid"
	default:
		return "Could not open channel"
	}
}

func renderLdkNoSwapError(c *gin.Context, message string) {
	c.Header("HX-Reswap", "none")
	c.Status(200)
	if renderErr := templates.ObbNotification(templates.ErrorNotif(message)).Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}

func renderLdkActionPanelError(c *gin.Context, message string) {
	c.Status(200)
	if renderErr := templates.LdkActionPanelErrorFragment(message).Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}

func renderLdkNoSwapSuccess(c *gin.Context, message string) {
	c.Header("HX-Reswap", "none")
	c.Status(200)
	if renderErr := templates.ObbNotification(templates.SuccessNotif(message)).Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}

func renderLdkNetworkSummaryError(c *gin.Context) {
	c.Status(200)
	if renderErr := templates.LdkNetworkSummaryErrorFragment("Could not load network summary").Render(c.Request.Context(), c.Writer); renderErr != nil {
		_ = c.Error(renderErr)
	}
}

func getLDKBackend(m *mint.Mint) (*ldk.LDK, error) {
	ldkBackend, ok := m.LightningBackend.(*ldk.LDK)
	if !ok {
		return nil, fmt.Errorf("expected LDK backend but got %T", m.LightningBackend)
	}
	return ldkBackend, nil
}

func formatLdkOnchainBalances(balances ldk.LDKBalances) (string, string) {
	totalOnchainBalance := templates.FormatNumber(balances.TotalOnchainSats) + " sats"
	availableOnchainBalance := templates.FormatNumber(balances.AvailableOnchainSats) + " sats"
	return totalOnchainBalance, availableOnchainBalance
}

func formatLdkLightningBalance(balances ldk.LDKBalances) string {
	return templates.FormatNumber(balances.LightningSats) + " sats"
}

func fetchLdkNetworkSummary(ldkBackend *ldk.LDK, channels []ldk.LDKChannelSummary) (ldkNetworkSummary, error) {
	peers, err := ldkPeerSummariesLoader(ldkBackend)
	if err != nil {
		slog.Error("could not fetch ldk peers for summary refresh",
			slog.String("event", "ldk_peers_summary_refresh_failed"),
			slog.String(utils.LogExtraInfo, err.Error()),
		)
		return ldkNetworkSummary{}, err
	}

	return mapLdkNetworkSummary(peers, channels), nil
}

func writeLdkMutationSuccessPayload(ctx context.Context, w io.Writer, rows []templates.LdkChannelRow, balances ldk.LDKBalances, balanceErr error, networkSummary ldkNetworkSummary, networkErr error, message string) error {
	if err := templates.LdkChannelsFragment(rows).Render(ctx, w); err != nil {
		return err
	}

	if balanceErr != nil {
		if err := templates.LdkBalancesErrorOOBFragment("Could not refresh LDK balances").Render(ctx, w); err != nil {
			return err
		}
	} else {
		totalOnchainBalance, availableOnchainBalance := formatLdkOnchainBalances(balances)
		if err := templates.LdkBalancesOOBFragment(totalOnchainBalance, availableOnchainBalance).Render(ctx, w); err != nil {
			return err
		}
	}

	if networkErr != nil {
		if err := templates.LdkNetworkSummaryErrorOOBFragment("Could not refresh network summary").Render(ctx, w); err != nil {
			return err
		}
	} else {
		lightningBalance := "Unavailable"
		if balanceErr == nil {
			lightningBalance = formatLdkLightningBalance(balances)
		}
		if err := templates.LdkNetworkSummaryOOBFragment(lightningBalance, networkSummary.TotalPeers, networkSummary.ActivePeers, networkSummary.TotalChannels, networkSummary.ActiveChannels).Render(ctx, w); err != nil {
			return err
		}
	}

	if err := templates.ObbNotification(templates.SuccessNotif(message)).Render(ctx, w); err != nil {
		return err
	}

	return nil
}

func writeLdkOnchainSendSuccessPayload(ctx context.Context, w io.Writer, balances ldk.LDKBalances, message string) error {
	totalOnchainBalance, availableOnchainBalance := formatLdkOnchainBalances(balances)
	if err := templates.LdkBalancesOOBFragment(totalOnchainBalance, availableOnchainBalance).Render(ctx, w); err != nil {
		return err
	}

	if err := templates.LdkOnchainSendSubmittedOOBFragment().Render(ctx, w); err != nil {
		return err
	}

	if err := templates.ObbNotification(templates.SuccessNotif(message)).Render(ctx, w); err != nil {
		return err
	}

	return nil
}
