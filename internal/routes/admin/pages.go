package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
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

		// Calculate default date range (7 weeks ago to now)
		now := time.Now()
		startTime := now.Add(-2 * 7 * 24 * time.Hour) // 7 weeks ago

		startDate := startTime.Format("2006-01-02")
		endDate := now.Format("2006-01-02")

		err := templates.LightningActivityLayout(startDate, endDate).Render(ctx, c.Writer)

		if err != nil {
			c.Error(err)
			return
		}
	}
}
