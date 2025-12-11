package admin

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func LoginPage(mint *mint.Mint, adminNostrKeyAvailable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// generate nonce for login nostr
		nonce, err := cashu.GenerateNonceHex()
		if err != nil {
			slog.Error(
				"database.SaveNostrLoginAuth(pool, nostrLogin)",
				slog.String(utils.LogExtraInfo, err.Error()))
			c.Error(err)
			return
		}

		nostrLogin := database.NostrLoginAuth{
			Nonce:     nonce,
			Expiry:    int(cashu.ExpiryTimeMinUnit(15)),
			Activated: false,
		}

		err = mint.MintDB.SaveNostrAuth(nostrLogin)
		if err != nil {
			slog.Error(
				"database.SaveNostrLoginAuth(pool, nostrLogin)",
				slog.String(utils.LogExtraInfo, err.Error()))
			c.Error(err)
			return

		}

		ctx := context.Background()
		err = templates.LoginPage(nonce, adminNostrKeyAvailable).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}

	}
}

func InitPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Default time range is 1 week
		selectedRange := "1w"

		err := templates.MintActivityLayout(
			utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND),
			selectedRange,
		).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			return
		}
	}
}

// parseTimeRange converts a time range string to a start time and bucket size
// Valid values: "1w" (7 days), "1m" (30 days), "3m" (90 days), "1y" (365 days), "all" (all time)
func parseTimeRange(timeRange string) (startTime time.Time, bucketMinutes int) {
	now := time.Now()

	switch timeRange {
	case "1w":
		startTime = now.Add(-7 * 24 * time.Hour)
		bucketMinutes = 60 // 1 hour buckets for 1 week
	case "1m":
		startTime = now.Add(-30 * 24 * time.Hour)
		bucketMinutes = 180 // 3 hour buckets for 1 month
	case "3m":
		startTime = now.Add(-90 * 24 * time.Hour)
		bucketMinutes = 360 // 6 hour buckets for 3 months
	case "1y":
		startTime = now.Add(-365 * 24 * time.Hour)
		bucketMinutes = 1440 // 24 hour buckets for 1 year
	case "all":
		startTime = time.Unix(0, 0)
		bucketMinutes = 1440 // 24 hour buckets for all time
	default:
		// Default to 1 week
		startTime = now.Add(-7 * 24 * time.Hour)
		bucketMinutes = 60
	}

	return startTime, bucketMinutes
}

// calculateChartSummary gets the latest data point values from time series data
// The data is sorted by timestamp, so the last item is the most recent
func calculateChartSummary(data []database.ProofTimeSeriesPoint) templates.ChartSummary {
	if len(data) == 0 {
		return templates.ChartSummary{
			TotalSats:  0,
			TotalCount: 0,
		}
	}
	// Get the last (most recent) data point
	latest := data[len(data)-1]
	return templates.ChartSummary{
		TotalSats:  latest.TotalAmount,
		TotalCount: latest.Count,
	}
}

// ProofsChartCard returns the full chart card component (for HTMX load with optional date params)
func ProofsChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		// Fetch proofs time-series data (use nil for until to get all data up to now)
		data, err := mint.MintDB.GetProofsTimeSeries(startTime.Unix(), bucketMinutes)
		if err != nil {
			slog.Error(
				"mint.MintDB.GetProofsTimeSeries()",
				slog.String(utils.LogExtraInfo, err.Error()))
			data = []database.ProofTimeSeriesPoint{}
		}

		summary := calculateChartSummary(data)

		err = templates.ProofsChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// ProofsChartDataAPI returns HTML fragment for the proofs chart based on time range (for HTMX updates)
func ProofsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		data, err := mint.MintDB.GetProofsTimeSeries(startTime.Unix(), bucketMinutes)
		if err != nil {
			slog.Error(
				"mint.MintDB.GetProofsTimeSeries()",
				slog.String(utils.LogExtraInfo, err.Error()))
			// Return empty data on error
			data = []database.ProofTimeSeriesPoint{}
		}

		// Return HTML fragment for HTMX
		err = templates.ProofsChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// BlindSigsChartCard returns the full blind sigs chart card component (for HTMX load with optional date params)
func BlindSigsChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		// Fetch blind sigs time-series data (use nil for until to get all data up to now)
		data, err := mint.MintDB.GetBlindSigsTimeSeries(startTime.Unix(), bucketMinutes)
		if err != nil {
			slog.Error(
				"mint.MintDB.GetBlindSigsTimeSeries()",
				slog.String(utils.LogExtraInfo, err.Error()))
			data = []database.ProofTimeSeriesPoint{}
		}

		summary := calculateChartSummary(data)

		err = templates.BlindSigsChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// BlindSigsChartDataAPI returns HTML fragment for the blind sigs chart based on time range (for HTMX updates)
func BlindSigsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		data, err := mint.MintDB.GetBlindSigsTimeSeries(startTime.Unix(), bucketMinutes)
		if err != nil {
			slog.Error(
				"mint.MintDB.GetBlindSigsTimeSeries()",
				slog.String(utils.LogExtraInfo, err.Error()))
			// Return empty data on error
			data = []database.ProofTimeSeriesPoint{}
		}

		// Return HTML fragment for HTMX
		err = templates.BlindSigsChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

func LigthningLiquidityPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		err := templates.LiquidityDashboard().Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func SwapStatusPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		swapId := c.Param("swapId")
		tx, err := mint.MintDB.GetTx(ctx)

		if err != nil {
			slog.Debug(
				"Incorrect body",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			} else {
				err = mint.MintDB.Commit(context.Background(), tx)
				if err != nil {
					c.Error(fmt.Errorf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		swap, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			c.Error(err)
			return
		}
		amount := strconv.FormatUint(swap.Amount, 10)
		// generate qrCode
		qrcode, err := generateQR(swap.LightningInvoice)
		if err != nil {
			c.Error(fmt.Errorf("generateQR(swap.LightningInvoice). %w", err))
			return
		}

		var component templ.Component
		switch swap.Type {
		case utils.LiquidityIn:
			component = templates.LightningReceiveSummary(amount, swap.LightningInvoice, qrcode, swap.Id)
		case utils.LiquidityOut:
			component = templates.LightningSendSummary(amount, swap.LightningInvoice, swap.Id)

		}

		err = templates.SwapStatusPage(component).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func LnPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Default time range is 1 week
		selectedRange := "1w"

		err := templates.LightningActivityLayout(mint.Config, selectedRange).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			return
		}
	}
}

// calculateLnChartSummary calculates total mint and melt amounts from time series data
func calculateLnChartSummary(data []templates.MintMeltTimeSeriesPoint) templates.LnChartSummary {
	var totalMint, totalMelt int64
	for _, point := range data {
		totalMint += point.MintAmount
		totalMelt += point.MeltAmount
	}
	return templates.LnChartSummary{
		TotalMint: totalMint,
		TotalMelt: totalMelt,
		NetFlow:   totalMint - totalMelt,
	}
}

// buildMintMeltTimeSeries processes raw mint/melt data and aggregates into time buckets
// using zpay32 to decode invoice amounts
func buildMintMeltTimeSeries(mintMeltBalance database.MintMeltBalance, network *chaincfg.Params, startTime time.Time, bucketMinutes int) []templates.MintMeltTimeSeriesPoint {
	bucketSeconds := int64(bucketMinutes * 60)

	// Maps to aggregate data by bucket
	mintBuckets := make(map[int64]*templates.MintMeltTimeSeriesPoint)
	meltBuckets := make(map[int64]*templates.MintMeltTimeSeriesPoint)

	// Process mint requests - decode invoice to get amount
	for _, mintRequest := range mintMeltBalance.Mint {

		invoice, err := zpay32.Decode(mintRequest.Request, network)
		if err != nil {
			slog.Debug(
				"zpay32.Decode failed for mint request",
				slog.String(utils.LogExtraInfo, err.Error()),
				slog.String("quote", mintRequest.Quote))
			continue // Skip this request
		}

		amount := int64(invoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi))
		bucketTs := (mintRequest.SeenAt / bucketSeconds) * bucketSeconds

		if _, ok := mintBuckets[bucketTs]; !ok {
			mintBuckets[bucketTs] = &templates.MintMeltTimeSeriesPoint{Timestamp: bucketTs}
		}
		mintBuckets[bucketTs].MintAmount += amount
		mintBuckets[bucketTs].MintCount++
	}

	// Process melt requests - use Amount field if > 0, otherwise decode invoice
	for _, meltRequest := range mintMeltBalance.Melt {

		var amount int64
		if meltRequest.Amount > 0 {
			amount = int64(meltRequest.Amount)
		} else {
			// Try to decode invoice
			invoice, err := zpay32.Decode(meltRequest.Request, network)
			if err != nil {
				slog.Debug(
					"zpay32.Decode failed for melt request, skipping",
					slog.String(utils.LogExtraInfo, err.Error()),
					slog.String("quote", meltRequest.Quote))
				continue // Skip this request
			}
			amount = int64(invoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi))
		}

		bucketTs := (meltRequest.SeenAt / bucketSeconds) * bucketSeconds

		if _, ok := meltBuckets[bucketTs]; !ok {
			meltBuckets[bucketTs] = &templates.MintMeltTimeSeriesPoint{Timestamp: bucketTs}
		}
		meltBuckets[bucketTs].MeltAmount += amount
		meltBuckets[bucketTs].MeltCount++
	}

	// Collect all unique timestamps
	allTimestamps := make(map[int64]bool)
	for ts := range mintBuckets {
		allTimestamps[ts] = true
	}
	for ts := range meltBuckets {
		allTimestamps[ts] = true
	}

	// Convert to sorted slice
	timestamps := make([]int64, 0, len(allTimestamps))
	for ts := range allTimestamps {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// Build final result combining mint and melt data
	result := make([]templates.MintMeltTimeSeriesPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		point := templates.MintMeltTimeSeriesPoint{Timestamp: ts}
		if mintData, ok := mintBuckets[ts]; ok {
			point.MintAmount = mintData.MintAmount
			point.MintCount = mintData.MintCount
		}
		if meltData, ok := meltBuckets[ts]; ok {
			point.MeltAmount = meltData.MeltAmount
			point.MeltCount = meltData.MeltCount
		}
		result = append(result, point)
	}

	return result
}

// LnChartCard returns the full LN chart card component (for HTMX load with optional date params)
func LnChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		// Fetch raw mint/melt data
		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetMintMeltBalanceByTime()",
				slog.String(utils.LogExtraInfo, err.Error()))
			mintMeltBalance = database.MintMeltBalance{}
		}

		fmt.Printf("\n mintMeltBalance: %+v\n", mintMeltBalance)

		// Process and aggregate into time series using zpay32 for invoice decoding
		data := buildMintMeltTimeSeries(mintMeltBalance, mint.LightningBackend.GetNetwork(), startTime,  bucketMinutes)

		summary := calculateLnChartSummary(data)

		err = templates.LnChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// LnChartDataAPI returns HTML fragment for the LN chart based on time range (for HTMX updates)
func LnChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		// Fetch raw mint/melt data
		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetMintMeltBalanceByTime()",
				slog.String(utils.LogExtraInfo, err.Error()))
			mintMeltBalance = database.MintMeltBalance{}
		}

		// Process and aggregate into time series using zpay32 for invoice decoding
		data := buildMintMeltTimeSeries(mintMeltBalance, mint.LightningBackend.GetNetwork(), startTime,  bucketMinutes)

		// Return HTML fragment for HTMX
		err = templates.LnChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}
