package admin

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func MintBalance(mint *m.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		isFakeWallet := false

		if mint.Config.MINT_LIGHTNING_BACKEND == utils.FAKE_WALLET {
			isFakeWallet = true
		}
		proofsReserve, err := mint.MintDB.GetProofsMintReserve()

		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetProofsMintReserve(). %w", err))
			return
		}
		sigsReserve, err := mint.MintDB.GetBlindSigsMintReserve()

		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetProofsMintReserve(). %w", err))
			return
		}

		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {
			slog.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "There was a problem getting the balance",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		component := templates.MintBalance(milillisatBalance/1000, isFakeWallet, proofsReserve, sigsReserve)

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
			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
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

				errorMessage := ErrorNotif{

					Error: "Could not decode invoice",
				}

				c.HTML(200, "settings-error", errorMessage)
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

			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
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

			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
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
