package admin

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lightningnetwork/lnd/zpay32"
)

func MintBalance(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {

		if mint.Config.MINT_LIGHTNING_BACKEND == comms.FAKE_WALLET {
			c.HTML(200, "fake-wallet-balance", nil)
			return

		}

		milillisatBalance, err := mint.LightningComs.WalletBalance()
		if err != nil {

			errorMessage := ErrorNotif{
				Error: "There was a problem getting the balance",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		c.HTML(200, "node-balance", milillisatBalance/1000)
	}
}

func MintMeltSummary(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")

		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := database.GetMintMeltBalanceByTime(pool, timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			log.Println(err)
			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		mintMeltTotal := make(map[string]float64)
		mintMeltTotal["Mint"] += 0
		// sum up mint
		for _, mintRequest := range mintMeltBalance.Mint {
			invoice, err := zpay32.Decode(mintRequest.Request, &mint.Network)

			if err != nil {
				log.Println(fmt.Errorf("Could not decode invoice %w", err))
				errorMessage := ErrorNotif{

					Error: "Could not decode invoice",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			mintMeltTotal["Mint"] += invoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi)
		}

		// sum up melt amount
		for _, meltRequest := range mintMeltBalance.Melt {

			mintMeltTotal["Melt"] += float64(meltRequest.Amount)
		}
		mintMeltTotal["Melt"] = mintMeltTotal["Melt"] * -1

		// get net flows
		mintMeltTotal["Net"] = mintMeltTotal["Mint"] - mintMeltTotal["Melt"]

		c.HTML(200, "mint-melt-activity", mintMeltTotal)
	}
}
func MintMeltList(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeHeader := c.GetHeader("time")
		timeRequestDuration := ParseToTimeRequest(timeHeader)

		mintMeltBalance, err := database.GetMintMeltBalanceByTime(pool, timeRequestDuration.RollBackFromNow().Unix())

		if err != nil {
			log.Println(err)
			errorMessage := ErrorNotif{

				Error: "There was an error getting mint activity",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		mintMeltRequestVisual := ListMintMeltVisual{}

		// sum up mint
		for _, mintRequest := range mintMeltBalance.Mint {
			utc := time.Unix(mintRequest.SeenAt, 0).UTC().Format("2006-Jan-2  15:04:05 MST")

			mintMeltRequestVisual = append(mintMeltRequestVisual, MintMeltRequestVisual{
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

			mintMeltRequestVisual = append(mintMeltRequestVisual, MintMeltRequestVisual{
				Type:    "Melt",
				Unit:    meltRequest.Unit,
				Request: meltRequest.Request,
				Status:  string(meltRequest.State),
				SeenAt:  utc,
			})
		}

		sort.Sort(mintMeltRequestVisual)
		slices.Reverse(mintMeltRequestVisual)

		c.HTML(200, "mint-melt-list", mintMeltRequestVisual)
	}
}

type MintMeltRequestVisual struct {
	Type    string
	Unit    string
	Request string
	Status  string
	SeenAt  string
}

type ListMintMeltVisual []MintMeltRequestVisual

func (ms ListMintMeltVisual) Len() int {
	return len(ms)
}

func (ms ListMintMeltVisual) Less(i, j int) bool {
	return ms[i].SeenAt < ms[j].SeenAt
}

func (ms ListMintMeltVisual) Swap(i, j int) {
	ms[i], ms[j] = ms[j], ms[i]
}
