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

		// Calculate default date range (7 weeks ago to now)
		now := time.Now()
		startTime := now.Add(-2 * 7 * 24 * time.Hour) // 7 weeks ago

		startDate := startTime.Format("2006-01-02")
		endDate := now.Format("2006-01-02")

		err := templates.MintActivityLayout(
			utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND),
			startDate,
			endDate,
		).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			return
		}
	}
}

// parseDateRange parses start and end date query params and returns parsed times and bucket minutes
func parseDateRange(startDateStr, endDateStr string) (startTime, endTime time.Time, bucketMinutes int) {
	var err error

	// Parse start date in YYYY-MM-DD format (HTML date input format)
	if startDateStr != "" {
		startTime, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			slog.Warn("Invalid start date format", slog.String("date", startDateStr))
			// Default to 7 weeks ago
			startTime = time.Now().Add(-7 * 7 * 24 * time.Hour)
		}
	} else {
		// Default to 7 weeks ago
		startTime = time.Now().Add(-7 * 7 * 24 * time.Hour)
	}

	// Parse end date
	if endDateStr != "" {
		endTime, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			slog.Warn("Invalid end date format", slog.String("date", endDateStr))
			endTime = time.Now()
		} else {
			// Set to end of day
			endTime = endTime.Add(24*time.Hour - time.Second)
		}
	} else {
		endTime = time.Now()
	}

	// Calculate appropriate bucket size based on date range
	duration := endTime.Sub(startTime)
	switch {
	case duration <= 24*time.Hour:
		bucketMinutes = 30 // 30 min buckets for 1 day
	case duration <= 7*24*time.Hour:
		bucketMinutes = 60 // 1 hour buckets for up to 1 week
	case duration <= 30*24*time.Hour:
		bucketMinutes = 180 // 3 hour buckets for up to 1 month
	default:
		bucketMinutes = 360 // 6 hour buckets for longer periods
	}

	return startTime, endTime, bucketMinutes
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

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		// Fetch proofs time-series data
		endUnix := endTime.Unix()
		data, err := mint.MintDB.GetProofsTimeSeries(startTime.Unix(), &endUnix, bucketMinutes)
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

// ProofsChartDataAPI returns HTML fragment for the proofs chart based on date range (for HTMX date updates)
func ProofsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		endUnix := endTime.Unix()
		data, err := mint.MintDB.GetProofsTimeSeries(startTime.Unix(), &endUnix, bucketMinutes)
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

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		// Fetch blind sigs time-series data
		endUnix := endTime.Unix()
		data, err := mint.MintDB.GetBlindSigsTimeSeries(startTime.Unix(), &endUnix, bucketMinutes)
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

// BlindSigsChartDataAPI returns HTML fragment for the blind sigs chart based on date range (for HTMX date updates)
func BlindSigsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		endUnix := endTime.Unix()
		data, err := mint.MintDB.GetBlindSigsTimeSeries(startTime.Unix(), &endUnix, bucketMinutes)
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
		milillisatBalance, err := mint.LightningBackend.WalletBalance()
		if err != nil {

			slog.Warn(
				"mint.LightningComs.WalletBalance()",
				slog.String(utils.LogExtraInfo, err.Error()))

			c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
		amount := strconv.FormatUint(milillisatBalance/1000, 10)

		err = templates.LiquidityDashboard(c.Query("swapForm"), amount).Render(ctx, c.Writer)

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

		// Calculate default date range (1 week ago to now)
		now := time.Now()
		startTime := now.Add(-7 * 24 * time.Hour) // 1 week ago

		startDate := startTime.Format("2006-01-02")
		endDate := now.Format("2006-01-02")

		err := templates.LightningActivityLayout(startDate, endDate).Render(ctx, c.Writer)

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
func buildMintMeltTimeSeries(mintMeltBalance database.MintMeltBalance, network *chaincfg.Params, startTime, endTime time.Time, bucketMinutes int) []templates.MintMeltTimeSeriesPoint {
	bucketSeconds := int64(bucketMinutes * 60)

	// Maps to aggregate data by bucket
	mintBuckets := make(map[int64]*templates.MintMeltTimeSeriesPoint)
	meltBuckets := make(map[int64]*templates.MintMeltTimeSeriesPoint)

	// Process mint requests - decode invoice to get amount
	for _, mintRequest := range mintMeltBalance.Mint {
		// Filter by end time
		if mintRequest.SeenAt >= endTime.Unix() {
			continue
		}

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
		// Filter by end time
		if meltRequest.SeenAt >= endTime.Unix() {
			continue
		}

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

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		// Fetch raw mint/melt data
		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetMintMeltBalanceByTime()",
				slog.String(utils.LogExtraInfo, err.Error()))
			mintMeltBalance = database.MintMeltBalance{}
		}

		// Process and aggregate into time series using zpay32 for invoice decoding
		data := buildMintMeltTimeSeries(mintMeltBalance, mint.LightningBackend.GetNetwork(), startTime, endTime, bucketMinutes)

		summary := calculateLnChartSummary(data)

		err = templates.LnChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// LnChartDataAPI returns HTML fragment for the LN chart based on date range (for HTMX date updates)
func LnChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Parse date range from query params
		startDateStr := c.Query("start")
		endDateStr := c.Query("end")
		startTime, endTime, bucketMinutes := parseDateRange(startDateStr, endDateStr)

		// Fetch raw mint/melt data
		mintMeltBalance, err := mint.MintDB.GetMintMeltBalanceByTime(startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetMintMeltBalanceByTime()",
				slog.String(utils.LogExtraInfo, err.Error()))
			mintMeltBalance = database.MintMeltBalance{}
		}

		// Process and aggregate into time series using zpay32 for invoice decoding
		data := buildMintMeltTimeSeries(mintMeltBalance, mint.LightningBackend.GetNetwork(), startTime, endTime, bucketMinutes)

		// Return HTML fragment for HTMX
		err = templates.LnChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			c.Error(err)
			return
		}
	}
}
