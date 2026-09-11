package payment

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ridex/ridex-angola/internal/db"
)

// AnalyticsStore is the narrow read surface for payment analytics. db.Queries
// satisfies it directly; keeping it separate from the payment Store means the
// mutation fakes and flows stay untouched.
type AnalyticsStore interface {
	PaymentAnalyticsSummary(ctx context.Context, since pgtype.Timestamptz) (db.PaymentAnalyticsSummaryRow, error)
	PaymentDailyVolume(ctx context.Context, since pgtype.Timestamptz) ([]db.PaymentDailyVolumeRow, error)
	PaymentProviderBreakdown(ctx context.Context, since pgtype.Timestamptz) ([]db.PaymentProviderBreakdownRow, error)
}

// Analytics aggregates payment charges for admin dashboards. It is read-only
// and shares nothing mutable with the payment service.
type Analytics struct {
	store AnalyticsStore
}

// NewAnalytics builds the analytics reader over the generated queries.
func NewAnalytics(store AnalyticsStore) *Analytics { return &Analytics{store: store} }

// Window bounds: 30 days by default, clamped to 1..90 so a bad query cannot
// scan a year of charges and a browser cannot trigger a full-table scan.
const (
	DefaultAnalyticsDays = 30
	MaxAnalyticsDays     = 90
)

func clampAnalyticsDays(days int) int32 {
	switch {
	case days <= 0:
		return DefaultAnalyticsDays
	case days > MaxAnalyticsDays:
		return MaxAnalyticsDays
	default:
		return int32(days)
	}
}

// PaymentAnalyticsJSON is the response shape for GET /admin/analytics/payments.
type PaymentAnalyticsJSON struct {
	WindowDays int32                 `json:"windowDays"`
	Since      string                `json:"since"`
	Summary    PaymentSummaryJSON    `json:"summary"`
	Daily      []PaymentDailyJSON    `json:"daily"`
	Providers  []PaymentProviderJSON `json:"providers"`
}

// PaymentSummaryJSON is the windowed totals block. SuccessRate is completed
// over decided (completed + failed), rounded to three decimals; 0 when
// nothing has been decided yet.
type PaymentSummaryJSON struct {
	TotalCharges         int64   `json:"totalCharges"`
	CompletedCharges     int64   `json:"completedCharges"`
	FailedCharges        int64   `json:"failedCharges"`
	RefundedCharges      int64   `json:"refundedCharges"`
	InFlightCharges      int64   `json:"inFlightCharges"`
	CompletedVolumeCents int64   `json:"completedVolumeCents"`
	RefundedVolumeCents  int64   `json:"refundedVolumeCents"`
	SuccessRate          float64 `json:"successRate"`
}

// PaymentDailyJSON is one day of the trend series.
type PaymentDailyJSON struct {
	Day                  string `json:"day"`
	Charges              int64  `json:"charges"`
	CompletedCharges     int64  `json:"completedCharges"`
	CompletedVolumeCents int64  `json:"completedVolumeCents"`
	RefundedVolumeCents  int64  `json:"refundedVolumeCents"`
}

// PaymentProviderJSON is one provider's share of the window.
type PaymentProviderJSON struct {
	Provider             string `json:"provider"`
	Charges              int64  `json:"charges"`
	CompletedCharges     int64  `json:"completedCharges"`
	FailedCharges        int64  `json:"failedCharges"`
	CompletedVolumeCents int64  `json:"completedVolumeCents"`
}

// Report builds the analytics payload for the window ending now.
func (a *Analytics) Report(ctx context.Context, days int) (PaymentAnalyticsJSON, error) {
	window := clampAnalyticsDays(days)
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -int(window))
	sinceArg := pgtype.Timestamptz{Time: since, Valid: true}

	summaryRow, err := a.store.PaymentAnalyticsSummary(ctx, sinceArg)
	if err != nil {
		return PaymentAnalyticsJSON{}, fmt.Errorf("payment analytics summary: %w", err)
	}
	dailyRows, err := a.store.PaymentDailyVolume(ctx, sinceArg)
	if err != nil {
		return PaymentAnalyticsJSON{}, fmt.Errorf("payment daily volume: %w", err)
	}
	providerRows, err := a.store.PaymentProviderBreakdown(ctx, sinceArg)
	if err != nil {
		return PaymentAnalyticsJSON{}, fmt.Errorf("payment provider breakdown: %w", err)
	}

	out := PaymentAnalyticsJSON{
		WindowDays: window,
		Since:      since.Format(time.RFC3339),
		Summary: PaymentSummaryJSON{
			TotalCharges:         summaryRow.TotalCharges,
			CompletedCharges:     summaryRow.CompletedCharges,
			FailedCharges:        summaryRow.FailedCharges,
			RefundedCharges:      summaryRow.RefundedCharges,
			InFlightCharges:      summaryRow.InFlightCharges,
			CompletedVolumeCents: summaryRow.CompletedVolumeCents,
			RefundedVolumeCents:  summaryRow.RefundedVolumeCents,
		},
		Daily:     make([]PaymentDailyJSON, 0, len(dailyRows)),
		Providers: make([]PaymentProviderJSON, 0, len(providerRows)),
	}
	if decided := summaryRow.CompletedCharges + summaryRow.FailedCharges; decided > 0 {
		out.Summary.SuccessRate = math.Round(float64(summaryRow.CompletedCharges)/float64(decided)*1000) / 1000
	}
	for _, d := range dailyRows {
		out.Daily = append(out.Daily, PaymentDailyJSON{
			Day:                  d.Day.Time.Format("2006-01-02"),
			Charges:              d.Charges,
			CompletedCharges:     d.CompletedCharges,
			CompletedVolumeCents: d.CompletedVolumeCents,
			RefundedVolumeCents:  d.RefundedVolumeCents,
		})
	}
	for _, p := range providerRows {
		out.Providers = append(out.Providers, PaymentProviderJSON{
			Provider:             p.Provider,
			Charges:              p.Charges,
			CompletedCharges:     p.CompletedCharges,
			FailedCharges:        p.FailedCharges,
			CompletedVolumeCents: p.CompletedVolumeCents,
		})
	}
	return out, nil
}

// AnalyticsHandler serves the admin payment analytics endpoint.
type AnalyticsHandler struct {
	analytics *Analytics
}

// NewAnalyticsHandler wires the analytics report to HTTP.
func NewAnalyticsHandler(analytics *Analytics) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

// Report handles GET /admin/analytics/payments?days=N. The window is clamped
// server-side; an unparseable or missing days parameter falls back to the
// default window rather than erroring.
func (h *AnalyticsHandler) Report(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	report, err := h.analytics.Report(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build payment analytics"})
		return
	}
	c.JSON(http.StatusOK, report)
}
