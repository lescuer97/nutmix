package admin

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func MintBalance(handler *adminHandler) gin.HandlerFunc {

	return func(c *gin.Context) {

		balance, err := handler.getProofsBalance(time.Unix(0, 0))
		if err != nil {
			c.Error(fmt.Errorf("handler.getProofsBalance(time.Unix(0, 0)). %w", err))
			return
		}
		// proofsReserve, err := mint.MintDB.GetProofsInventory(time.Unix(0, 0), nil)
		//
		// if err != nil {
		// 	c.Error(fmt.Errorf("mint.MintDB.GetProofsMintReserve(). %w", err))
		// 	return
		// }
		// sigsReserve, err := mint.MintDB.GetBlindSigsInventory(time.Unix(0, 0), nil)
		//
		// if err != nil {
		// 	c.Error(fmt.Errorf("mint.MintDB.GetProofsMintReserve(). %w", err))
		// 	return
		// }

		milillisatBalance, err := handler.lnSatsBalance()
		if err != nil {
			slog.Warn(
				"handler.lnSatsBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			err := RenderError(c, "There was a problem getting the balance")
			if err != nil {
				slog.Error("RenderError", slog.Any("error", err))
			}
			return
		}

		component := templates.MintBalance(milillisatBalance, handler.isFakeWallet(), balance)

		err = component.Render(c.Request.Context(), c.Writer)
		if err != nil {
			c.Error(err)
			c.Status(400)
			return
		}
	}
}

func MintMeltSummary(mint *m.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			slog.Error(
				"database.GetMintMeltBalanceByTime(pool",
				slog.String(utils.LogExtraInfo, err.Error()))
			err := RenderError(c, "There was an error getting mint activity")
			if err != nil {
				slog.Error("RenderError", slog.Any("error", err))
			}
			return
		}

		activitySummary := templates.ActivitySummary{
			Mint: 0,
			Melt: 0,
			Net:  0,
		}
		// sum up mint
		for _, mintRequest := range mintMeltBalance.Mint {
			invoice, err := zpay32.Decode(mintRequest.Request, mint.LightningBackend.GetNetwork())

			if err != nil {
				slog.Debug(
					"zpay32.Decode",
					slog.String(utils.LogExtraInfo, err.Error()))

				err := RenderError(c, "Could not decode invoice")
				if err != nil {
					slog.Error("RenderError", slog.Any("error", err))
				}
				return
			}

			activitySummary.Mint += int64(invoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi))
		}

		// sum up melt amount
		for _, meltRequest := range mintMeltBalance.Melt {

			activitySummary.Melt += int64(meltRequest.Amount)
		}
		activitySummary.Melt = activitySummary.Melt * -1

		// get net flows
		activitySummary.Net = activitySummary.Mint + activitySummary.Mint

		err = templates.MintMovements(activitySummary).Render(context.Background(), c.Writer)
		if err != nil {
			c.Error(err)
			c.Status(400)
			return
		}
	}
}
func MintMeltList(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")
		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			slog.Error(
				"database.GetMintMeltBalanceByTime(pool",
				slog.String(utils.LogExtraInfo, err.Error()))

			err := RenderError(c, "There was an error getting mint activity")
			if err != nil {
				slog.Error("RenderError", slog.Any("error", err))
			}
			return
		}

		mintMeltRequestVisual := templates.ListMintMeltVisual{}

		// sum up mint
		for _, mintRequest := range mintMeltBalance.Mint {
			utc := time.Unix(mintRequest.SeenAt, 0).UTC().Format("2006-Jan-2  15:04:05 MST")

			mintMeltRequestVisual = append(mintMeltRequestVisual, templates.MintMeltRequestVisual{
				Type:    "Mint",
				Unit:    mintRequest.Unit,
				Request: mintRequest.Request,
				Status:  string(mintRequest.State),
				SeenAt:  utc,
			})

		}

		// sum up melt amount
		for _, meltRequest := range mintMeltBalance.Melt {
			utc := time.Unix(meltRequest.SeenAt, 0).UTC().Format("2006-Jan-2  15:04:05 MST")

			mintMeltRequestVisual = append(mintMeltRequestVisual, templates.MintMeltRequestVisual{
				Type:    "Melt",
				Unit:    meltRequest.Unit,
				Request: meltRequest.Request,
				Status:  string(meltRequest.State),
				SeenAt:  utc,
			})
		}

		sort.Sort(mintMeltRequestVisual)

		ctx := context.Background()

		err = templates.MintMeltEventList(mintMeltRequestVisual).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			c.Status(400)
			return
		}
	}
}

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

		ctx := context.Background()

		err = templates.ListOfSwaps(swaps).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
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

		proofsCount, err := adminHandler.getProofsCountByKeyset(startTime, nil)
		if err != nil {
			c.Error(err)
			return
		}

		keysets, err := mint.Signer.GetKeysets()
		if err != nil {
			c.Error(err)
			return
		}

		fees, err := fees(proofsCount, keysets.Keysets)
		if err != nil {
			c.Error(err)
			return
		}

		lnBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {
			c.Error(err)
			return
		}

		// Format the since date for display
		sinceDate := startTime.Format("Jan 2, 2006")
		if timeRange == "all" {
			sinceDate = "the beginning"
		}

		summary := templates.Summary{
			LnBalance:  lnBalance / 1000,
			FakeWallet: mint.Config.MINT_LIGHTNING_BACKEND == utils.FAKE_WALLET,
			Fees:       fees,
			SinceDate:  sinceDate,
		}

		err = templates.SummaryComponent(summary).Render(c.Request.Context(), c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

func fees(proofs map[string]database.ProofsCountByKeyset, keysets []cashu.BasicKeysetResponse) (uint64, error) {
	totalFees := uint64(0)

	for _, keyset := range keysets {
		if keyset.Unit != cashu.AUTH.String() {
			for keysetId, proof := range proofs {
				if keyset.Id == keysetId {
					totalFees += uint64(proof.Count) * uint64(keyset.InputFeePpk)
				}
			}
		}

	}

	totalFees = (totalFees + 999) / 1000

	return totalFees, nil

}
