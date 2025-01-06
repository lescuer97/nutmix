package admin

import (
	"context"
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

func MintBalance(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {

		if mint.Config.MINT_LIGHTNING_BACKEND == utils.FAKE_WALLET {
			c.HTML(200, "fake-wallet-balance", nil)
			return

		}

		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {

			logger.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "There was a problem getting the balance",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		c.HTML(200, "node-balance", milillisatBalance/1000)
	}
}

func MintMeltSummary(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			logger.Error(
				"database.GetMintMeltBalanceByTime(pool",
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		mintMeltTotal := make(map[string]int64)
		mintMeltTotal["Mint"] += 0
		// sum up mint
		for _, mintRequest := range mintMeltBalance.Mint {
			invoice, err := zpay32.Decode(mintRequest.Request, mint.LightningBackend.GetNetwork())

			if err != nil {
				logger.Debug(
					"zpay32.Decode",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{

					Error: "Could not decode invoice",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			mintMeltTotal["Mint"] += int64(invoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi))
		}

		// sum up melt amount
		for _, meltRequest := range mintMeltBalance.Melt {

			mintMeltTotal["Melt"] += int64(meltRequest.Amount)
		}
		mintMeltTotal["Melt"] = mintMeltTotal["Melt"] * -1

		// get net flows
		mintMeltTotal["Net"] = mintMeltTotal["Mint"] + mintMeltTotal["Melt"]

		c.HTML(200, "mint-melt-activity", mintMeltTotal)
	}
}
func MintMeltList(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")
		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			logger.Error(
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

func SwapsList(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		swaps, err := mint.MintDB.GetAllLiquiditySwaps()

		if err != nil {
			logger.Error(
				"database.GetMintMeltBalanceByTime(pool",
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
