package admin

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

func LdkOnchainSendFormFragment(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkActionPanelError(c, "Could not load on-chain send form")
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances for on-chain send form",
				slog.String("event", "ldk_onchain_send_form_balance_fetch_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkActionPanelError(c, "Could not load on-chain send form")
			return
		}

		c.Status(200)
		if renderErr := templates.LdkOnchainSendFormFragment(balances.AvailableOnchainSats).Render(c.Request.Context(), c.Writer); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func LdkSendOnchain(m *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		address := strings.TrimSpace(c.PostForm("bitcoin_address"))
		amountText := strings.TrimSpace(c.PostForm("sats_amount"))

		ldkBackend, err := getLDKBackend(m)
		if err != nil {
			renderLdkNoSwapError(c, "Could not send on-chain payment")
			return
		}

		satsAmount, err := parseLdkOnchainSendAmount(amountText)
		if err != nil {
			renderLdkNoSwapError(c, displayLdkValidationError(err))
			return
		}

		if err := ldkBackend.SendOnchain(address, satsAmount); err != nil {
			if ldk.IsOnchainSendValidationError(err) {
				message := strings.TrimPrefix(err.Error(), "on-chain send validation failed: ")
				renderLdkNoSwapError(c, displayLdkValidationError(fmt.Errorf("%s", message)))
				return
			}
			slog.Error("ldk on-chain send failed",
				slog.String("event", "ldk_onchain_send_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapError(c, mapLdkOnchainSendError(err))
			return
		}

		balances, err := ldkBackend.Balances()
		if err != nil {
			slog.Error("could not fetch ldk balances after on-chain send",
				slog.String("event", "ldk_onchain_send_balance_refresh_failed"),
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			renderLdkNoSwapSuccess(c, "On-chain payment sent, but balances could not be refreshed")
			return
		}

		c.Header("HX-Reswap", "none")
		c.Status(200)
		if renderErr := writeLdkOnchainSendSuccessPayload(c.Request.Context(), c.Writer, balances, "On-chain payment sent"); renderErr != nil {
			_ = c.Error(renderErr)
		}
	}
}

func parseLdkOnchainSendAmount(raw string) (uint64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("sats amount is required")
	}

	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("sats amount must be a positive integer")
	}

	return amount, nil
}

func mapLdkOnchainSendError(err error) string {
	lower := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lower, "insufficient"), strings.Contains(lower, "spendable"), strings.Contains(lower, "balance"), strings.Contains(lower, "fund"), strings.Contains(lower, "reserve"):
		return "Insufficient available on-chain balance to send funds"
	case strings.Contains(lower, "invalid address"), strings.Contains(lower, "address"), strings.Contains(lower, "bech32"):
		return "Destination Bitcoin address is invalid"
	default:
		return "Could not create or broadcast on-chain payment"
	}
}
