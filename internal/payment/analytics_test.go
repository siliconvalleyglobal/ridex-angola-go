package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ridex/ridex-angola/internal/db"
)

// fakeAnalyticsStore records the since argument and returns canned rows.
type fakeAnalyticsStore struct {
	since     pgtype.Timestamptz
	summary   db.PaymentAnalyticsSummaryRow
	daily     []db.PaymentDailyVolumeRow
	providers []db.PaymentProviderBreakdownRow
	err       error
}

func (f *fakeAnalyticsStore) PaymentAnalyticsSummary(_ context.Context, since pgtype.Timestamptz) (db.PaymentAnalyticsSummaryRow, error) {
	f.since = since
	if f.err != nil {
		return db.PaymentAnalyticsSummaryRow{}, f.err
	}
	return f.summary, nil
}

func (f *fakeAnalyticsStore) PaymentDailyVolume(_ context.Context, _ pgtype.Timestamptz) ([]db.PaymentDailyVolumeRow, error) {
	return f.daily, nil
}

func (f *fakeAnalyticsStore) PaymentProviderBreakdown(_ context.Context, _ pgtype.Timestamptz) ([]db.PaymentProviderBreakdownRow, error) {
	return f.providers, nil
}

func TestAnalyticsReportComputesSuccessRateAndShapes(t *testing.T) {
	store := &fakeAnalyticsStore{
		summary: db.PaymentAnalyticsSummaryRow{
			TotalCharges: 100, CompletedCharges: 88, FailedCharges: 12, RefundedCharges: 5,
			InFlightCharges: 3, CompletedVolumeCents: 880000, RefundedVolumeCents: 25000,
		},
		daily: []db.PaymentDailyVolumeRow{
			{Day: pgtype.Date{Time: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				Charges: 40, CompletedCharges: 37, CompletedVolumeCents: 370000, RefundedVolumeCents: 5000},
		},
		providers: []db.PaymentProviderBreakdownRow{
			{Provider: "appypay", Charges: 70, CompletedCharges: 65, FailedCharges: 5, CompletedVolumeCents: 650000},
			{Provider: "vpos", Charges: 30, CompletedCharges: 23, FailedCharges: 7, CompletedVolumeCents: 230000},
		},
	}
	report, err := NewAnalytics(store).Report(context.Background(), 14)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.WindowDays != 14 {
		t.Fatalf("windowDays = %d, want 14", report.WindowDays)
	}
	if _, err := time.Parse(time.RFC3339, report.Since); err != nil {
		t.Fatalf("since %q is not RFC3339: %v", report.Since, err)
	}
	// 88 completed of 100 decided (88+12): 0.88.
	if report.Summary.SuccessRate != 0.88 {
		t.Fatalf("successRate = %v, want 0.88", report.Summary.SuccessRate)
	}
	if report.Summary.CompletedVolumeCents != 880000 || report.Summary.RefundedVolumeCents != 25000 {
		t.Fatalf("summary volumes = %#v", report.Summary)
	}
	if len(report.Daily) != 1 || report.Daily[0].Day != "2026-09-10" || report.Daily[0].CompletedCharges != 37 {
		t.Fatalf("daily = %#v", report.Daily)
	}
	if len(report.Providers) != 2 || report.Providers[0].Provider != "appypay" {
		t.Fatalf("providers = %#v", report.Providers)
	}
}

func TestAnalyticsReportEmptyWindowHasZeroRateAndEmptySeries(t *testing.T) {
	report, err := NewAnalytics(&fakeAnalyticsStore{}).Report(context.Background(), 30)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.Summary.SuccessRate != 0 || report.Summary.TotalCharges != 0 {
		t.Fatalf("summary = %#v, want zeroed", report.Summary)
	}
	if report.Daily == nil || report.Providers == nil {
		t.Fatal("series should be empty JSON arrays, not null")
	}
}

func TestAnalyticsReportSurfacesStoreError(t *testing.T) {
	if _, err := NewAnalytics(&fakeAnalyticsStore{err: errors.New("db down")}).Report(context.Background(), 7); err == nil {
		t.Fatal("store error should surface")
	}
}

func TestPayoutClampWindow(t *testing.T) {
	cases := map[int]int32{0: DefaultAnalyticsDays, -5: DefaultAnalyticsDays, 7: 7, 500: MaxAnalyticsDays}
	for days, want := range cases {
		if got := clampAnalyticsDays(days); got != want {
			t.Fatalf("clampAnalyticsDays(%d) = %d, want %d", days, got, want)
		}
	}
}

func TestAnalyticsHandlerServesReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeAnalyticsStore{
		summary:   db.PaymentAnalyticsSummaryRow{TotalCharges: 10, CompletedCharges: 10, CompletedVolumeCents: 50000},
		providers: []db.PaymentProviderBreakdownRow{{Provider: "appypay", Charges: 10, CompletedCharges: 10, CompletedVolumeCents: 50000}},
	}
	handler := NewAnalyticsHandler(NewAnalytics(store))
	router := gin.New()
	router.GET("/admin/analytics/payments", handler.Report)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/analytics/payments?days=7", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
	var out PaymentAnalyticsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.WindowDays != 7 || out.Summary.CompletedCharges != 10 || len(out.Providers) != 1 {
		t.Fatalf("report = %#v", out)
	}
}
