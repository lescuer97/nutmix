package admin

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
	"golang.org/x/sync/errgroup"
)

const lightningSearchLimit = 200
const minLightningSearchLength = 2

func LoginPage(mint *mint.Mint, adminNostrKeyAvailable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// generate nonce for login nostr
		nonce, err := cashu.GenerateNonceHex()
		if err != nil {
			slog.Error(
				"database.SaveNostrLoginAuth(pool, nostrLogin)",
				slog.String(utils.LogExtraInfo, err.Error()))
			_ = c.Error(err)
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
			_ = c.Error(err)
			return
		}

		ctx := c.Request.Context()
		err = templates.LoginPage(nonce, adminNostrKeyAvailable).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func InitPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Default time range is 1 week
		selectedRange := "1w"

		err := templates.MintActivityLayout(
			utils.CanUseLiquidityManager(mint.Config.MINT_LIGHTNING_BACKEND),
			selectedRange,
		).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(err)
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
	totalSats := uint64(0)
	totalCount := uint64(0)
	for _, point := range data {
		totalSats += point.TotalAmount
		totalCount += point.Count
	}
	return templates.ChartSummary{
		TotalSats:  totalSats,
		TotalCount: totalCount,
	}
}

// ProofsChartCard returns the full chart card component (for HTMX load with optional date params)
func ProofsChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildProofTimeSeriesFromStats(statsRows, func(row database.StatsSnapshot) []database.StatsSummaryItem {
			return row.ProofsSummary
		}, bucketMinutes)

		summary := calculateChartSummary(data)

		err = templates.ProofsChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

// ProofsChartDataAPI returns HTML fragment for the proofs chart based on time range (for HTMX updates)
func ProofsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildProofTimeSeriesFromStats(statsRows, func(row database.StatsSnapshot) []database.StatsSummaryItem {
			return row.ProofsSummary
		}, bucketMinutes)

		// Return HTML fragment for HTMX
		err = templates.ProofsChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

// BlindSigsChartCard returns the full blind sigs chart card component (for HTMX load with optional date params)
func BlindSigsChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildProofTimeSeriesFromStats(statsRows, func(row database.StatsSnapshot) []database.StatsSummaryItem {
			return row.BlindSigsSummary
		}, bucketMinutes)

		summary := calculateChartSummary(data)

		err = templates.BlindSigsChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

// BlindSigsChartDataAPI returns HTML fragment for the blind sigs chart based on time range (for HTMX updates)
func BlindSigsChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildProofTimeSeriesFromStats(statsRows, func(row database.StatsSnapshot) []database.StatsSummaryItem {
			return row.BlindSigsSummary
		}, bucketMinutes)

		// Return HTML fragment for HTMX
		err = templates.BlindSigsChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func LigthningLiquidityPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		err := templates.LiquidityDashboard().Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func SwapStatusPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		swapId := c.Param("swapId")
		tx, err := mint.MintDB.GetTx(ctx)

		if err != nil {
			slog.Debug(
				"Incorrect body",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			_ = c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				_ = c.Error(fmt.Errorf("rolling back because of failure %+v", err))
				if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
					slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
				}
			} else if err != nil {
				_ = c.Error(fmt.Errorf("rolling back because of failure %+v", err))
				if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
					slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
				}
			}
		}()

		swap, err := mint.MintDB.GetLiquiditySwapById(tx, swapId)
		if err != nil {
			_ = c.Error(err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			_ = c.Error(fmt.Errorf("tx.Commit failed: %w", err))
			return
		}
		amount := strconv.FormatUint(swap.Amount, 10)
		// generate qrCode
		qrcode, err := generateQR(swap.LightningInvoice)
		if err != nil {
			_ = c.Error(fmt.Errorf("generateQR(swap.LightningInvoice). %w", err))
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
			_ = c.Error(err)
			// c.HTML(400,"", nil)
			return
		}
	}
}

func LnPage(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Default time range is 1 week
		selectedRange := c.DefaultQuery("since", "1w")
		searchQuery := strings.TrimSpace(c.Query("search"))

		err := templates.LightningActivityLayout(mint.Config, selectedRange, searchQuery).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

func LightningTable(adminHandler *adminHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		searchQuery := strings.TrimSpace(c.Query("search"))
		timeRange := c.Query("since")
		startTime, _ := parseTimeRange(timeRange)

		mintRequests := make([]cashu.MintRequestDB, 0)
		meltRequests := make([]cashu.MeltRequestDB, 0)
		filtered := make([]templates.LightningInvoiceVisual, 0)

		if len([]rune(searchQuery)) < minLightningSearchLength {
			errGroup := errgroup.Group{}
			errGroup.Go(func() error {
				requests, err := adminHandler.mint.MintDB.GetMintRequestsByTime(ctx, startTime)
				if err != nil {
					return err
				}
				mintRequests = requests
				return nil
			})
			errGroup.Go(func() error {
				requests, err := adminHandler.mint.MintDB.GetMeltRequestsByTime(ctx, startTime)
				if err != nil {
					return err
				}
				meltRequests = requests
				return nil
			})
			err := errGroup.Wait()
			if err != nil {
				_ = c.Error(err)
				return
			}

			filtered = make([]templates.LightningInvoiceVisual, 0, len(mintRequests)+len(meltRequests))
			for _, mintRequest := range mintRequests {
				filtered = append(filtered, templates.LightningInvoiceVisual{
					Id:      mintRequest.Quote,
					Type:    "mint",
					Invoice: mintRequest.Request,
					Status:  string(mintRequest.State),
					Unit:    mintRequest.Unit,
					Time:    mintRequest.SeenAt,
				})
			}
			for _, meltRequest := range meltRequests {
				filtered = append(filtered, templates.LightningInvoiceVisual{
					Id:      meltRequest.Quote,
					Type:    "melt",
					Invoice: meltRequest.Request,
					Status:  string(meltRequest.State),
					Unit:    meltRequest.Unit,
					Time:    meltRequest.SeenAt,
				})
			}
		} else {
			searchRows, err := adminHandler.mint.MintDB.SearchLightningRequests(ctx, searchQuery, startTime, lightningSearchLimit)
			if err != nil {
				_ = c.Error(err)
				return
			}

			filtered = make([]templates.LightningInvoiceVisual, 0, len(searchRows))
			for _, row := range searchRows {
				filtered = append(filtered, templates.LightningInvoiceVisual{
					Id:      row.ID,
					Type:    row.Type,
					Invoice: row.Request,
					Status:  row.State,
					Unit:    row.Unit,
					Time:    row.SeenAt,
				})
			}
		}

		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Time > filtered[j].Time })

		err := templates.LightningActivityTable(filtered).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
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

func sumStatsSummary(items []database.StatsSummaryItem) (int64, uint64) {
	var amount int64
	var count uint64
	for _, item := range items {
		amount += int64(item.Amount)
		count += item.Quantity
	}
	return amount, count
}

func statsBucketTimestamp(endDate int64, bucketMinutes int) int64 {
	bucketSeconds := int64(bucketMinutes * 60)
	if bucketSeconds <= 0 {
		return endDate
	}
	return (endDate / bucketSeconds) * bucketSeconds
}

func buildMintMeltTimeSeriesFromStats(rows []database.StatsSnapshot, bucketMinutes int) []templates.MintMeltTimeSeriesPoint {
	buckets := make(map[int64]*templates.MintMeltTimeSeriesPoint)
	for _, row := range rows {
		bucketTs := statsBucketTimestamp(row.EndDate, bucketMinutes)
		point, ok := buckets[bucketTs]
		if !ok {
			point = &templates.MintMeltTimeSeriesPoint{
				Timestamp:  bucketTs,
				MintAmount: 0,
				MeltAmount: 0,
				MintCount:  0,
				MeltCount:  0,
			}
			buckets[bucketTs] = point
		}
		mintAmount, mintCount := sumStatsSummary(row.MintSummary)
		meltAmount, meltCount := sumStatsSummary(row.MeltSummary)
		point.MintAmount += mintAmount
		point.MintCount += mintCount
		point.MeltAmount += meltAmount
		point.MeltCount += meltCount
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	result := make([]templates.MintMeltTimeSeriesPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		result = append(result, *buckets[ts])
	}
	return result
}

func buildProofTimeSeriesFromStats(rows []database.StatsSnapshot, selector func(database.StatsSnapshot) []database.StatsSummaryItem, bucketMinutes int) []database.ProofTimeSeriesPoint {
	buckets := make(map[int64]*database.ProofTimeSeriesPoint)
	for _, row := range rows {
		bucketTs := statsBucketTimestamp(row.EndDate, bucketMinutes)
		point, ok := buckets[bucketTs]
		if !ok {
			point = &database.ProofTimeSeriesPoint{
				Timestamp:   bucketTs,
				TotalAmount: 0,
				Count:       0,
			}
			buckets[bucketTs] = point
		}
		amount, count := sumStatsSummary(selector(row))
		point.TotalAmount += uint64(amount)
		point.Count += count
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	result := make([]database.ProofTimeSeriesPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		result = append(result, *buckets[ts])
	}
	return result
}

// LnChartCard returns the full LN chart card component (for HTMX load with optional date params)
func LnChartCard(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildMintMeltTimeSeriesFromStats(statsRows, bucketMinutes)

		summary := calculateLnChartSummary(data)

		err = templates.LnChartCard(data, summary).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}

// LnChartDataAPI returns HTML fragment for the LN chart based on time range (for HTMX updates)
func LnChartDataAPI(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse time range from query params
		timeRange := c.Query("since")
		startTime, bucketMinutes := parseTimeRange(timeRange)

		statsRows, err := mint.MintDB.GetStatsSnapshotsBySince(ctx, startTime.Unix())
		if err != nil {
			slog.Error(
				"mint.MintDB.GetStatsSnapshotsBySince()",
				slog.String(utils.LogExtraInfo, err.Error()))
			statsRows = []database.StatsSnapshot{}
		}

		data := buildMintMeltTimeSeriesFromStats(statsRows, bucketMinutes)

		// Return HTML fragment for HTMX
		err = templates.LnChartContent(data).Render(ctx, c.Writer)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}
}
