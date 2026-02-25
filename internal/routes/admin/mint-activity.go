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

		proofsCount, err := adminHandler.getProofsCountByKeyset(startTime)
		if err != nil {
			_ = c.Error(err)
			return
		}

		keysets, err := mint.Signer.GetKeysets()
		if err != nil {
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

		summary := templates.Summary{
			LnBalance:  lnBalance,
			FakeWallet: mint.Config.MINT_LIGHTNING_BACKEND == utils.FAKE_WALLET,
			Fees:       fees(proofsCount, keysets.Keysets),
			SinceDate:  sinceDate,
		}

		err = templates.SummaryComponent(summary).Render(c.Request.Context(), c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func fees(proofs map[string]database.ProofsCountByKeyset, keysets []cashu.BasicKeysetResponse) uint64 {
	totalFees := uint64(0)

	for _, keyset := range keysets {
		if keyset.Unit != cashu.AUTH.String() {
			for keysetId, proof := range proofs {
				if keyset.Id == keysetId {
					totalFees += proof.Count * uint64(keyset.InputFeePpk)
				}
			}
		}

	}

	totalFees = (totalFees + 999) / 1000

	return totalFees
}
