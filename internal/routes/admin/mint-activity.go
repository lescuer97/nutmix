package admin

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

func SwapsList(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		swaps, err := mint.MintDB.GetAllLiquiditySwaps()

		if err != nil {
			slog.Error(
				"mint.MintDB.GetAllLiquiditySwaps()",
				slog.String(utils.LogExtraInfo, err.Error()))

			err := RenderError(c, "There was an error getting mint activity")
			if err != nil {
				slog.Error("RenderError", slog.Any("error", err))
			}
			return
		}

		ctx := c.Request.Context()

		err = templates.ListOfSwaps(swaps).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			c.Status(400)
			return
		}
	}
}

func SummaryComponent(mint *m.Mint, adminHandler *adminHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, _ := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(c.Request.Context(), startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			_ = c.Error(err)
			return
		}

		lnBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {
			_ = c.Error(err)
			return
		}

		// Format the since date for display
		sinceDate := startTime.Format("Jan 2, 2006")
		if timeRange == "all" {
			sinceDate = "the beginning"
		}

		summary := buildSummaryFromStats(statsRows, lnBalance, mint.Config.MINT_LIGHTNING_BACKEND == utils.FAKE_WALLET, sinceDate)

		err = templates.SummaryComponent(summary).Render(c.Request.Context(), c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func buildSummaryFromStats(rows []database.StatsSnapshot, lnBalance cashu.Amount, fakeWallet bool, sinceDate string) templates.Summary {
	return templates.Summary{
		LnBalance:  lnBalance,
		FakeWallet: fakeWallet,
		Fees:       sumFeesFromStats(rows),
		SinceDate:  sinceDate,
	}
}

func sumFeesFromStats(rows []database.StatsSnapshot) uint64 {
	totalFees := uint64(0)
	for _, row := range rows {
		totalFees += row.Fees
	}
	return totalFees
}
