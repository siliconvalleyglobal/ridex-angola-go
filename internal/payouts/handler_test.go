package payouts

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

func newTestRouter(ledger *Ledger, driverID uuid.UUID, isDriver bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(ledger)
	role := "driver"
	if !isDriver {
		role = "admin"
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.ContextUserID, driverID.String())
		c.Set(auth.ContextRole, role)
		c.Next()
	})
	router.GET("/driver/wallet", auth.RequireRole("driver"), handler.Wallet)
	router.GET("/driver/wallet/transactions", auth.RequireRole("driver"), handler.WalletTransactions)
	router.POST("/driver/payouts", auth.RequireRole("driver"), handler.RequestWithdrawal)
	router.GET("/driver/payouts", auth.RequireRole("driver"), handler.DriverPayouts)
	router.GET("/admin/payouts", auth.RequireRole("admin"), handler.AdminPayoutQueue)
	router.POST("/admin/payouts/:payoutId/approve", auth.RequireRole("admin"), handler.ApprovePayout)
	router.POST("/admin/payouts/:payoutId/complete", auth.RequireRole("admin"), handler.CompletePayout)
	router.POST("/admin/payouts/:payoutId/fail", auth.RequireRole("admin"), handler.FailPayout)
	return router
}

func assertStatus(t *testing.T, got, want int, body string) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d: %s", got, want, body)
	}
}

func TestDriverRequestWithdrawal(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 25000)
	ledger := NewLedger(store)
	router := newTestRouter(ledger, driverID, true)

	body := `{"amountCents":6000,"method":"multicada","notes":"airport"}`
	req := httptest.NewRequest(http.MethodPost, "/driver/payouts", bytes.NewReader([]byte(body)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assertStatus(t, res.Code, http.StatusCreated, res.Body.String())
	if store.deductCalls != 1 || store.wallet.BalanceCents != 19000 {
		t.Fatalf("wallet mutated wrong: deduct=%d balance=%d", store.deductCalls, store.wallet.BalanceCents)
	}
	if len(store.payouts) != 1 || store.payouts[0].Status != "pending" {
		t.Fatalf("payout request not created: %#v", store.payouts)
	}
}

func TestDriverRequestWithdrawalRejectsUnderMinimum(t *testing.T) {
	driverID := uuid.New()
	router := newTestRouter(NewLedger(walletStoreWith(driverID, 10000)), driverID, true)
	req := httptest.NewRequest(http.MethodPost, "/driver/payouts",
		bytes.NewReader([]byte(`{"amountCents":1000,"method":"bank"}`)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assertStatus(t, res.Code, http.StatusBadRequest, res.Body.String())
}

func TestDriverRequestWithdrawalRejectsInsufficientBalance(t *testing.T) {
	driverID := uuid.New()
	ledger := NewLedger(walletStoreWith(driverID, 1000)).WithMethods(NewPayoutMethods("multicada").WithInfo(
		PayoutMethodMulticada, PayoutMethodInfo{Label: "Multicaixa Express", Currency: "AOA", MinCents: 500, MaxCents: 500000}))
	router := newTestRouter(ledger, driverID, true)
	req := httptest.NewRequest(http.MethodPost, "/driver/payouts",
		bytes.NewReader([]byte(`{"amountCents":5000,"method":"multicada"}`)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assertStatus(t, res.Code, http.StatusUnprocessableEntity, res.Body.String())
}

func TestDriverRequestWithdrawalRejectsDuplicate(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 30000)
	store.payouts = append(store.payouts, db.PayoutRequest{
		ID: uuid.New(), DriverID: driverID, AmountCents: 5000, Method: "bank", Status: "processing",
	})
	router := newTestRouter(NewLedger(store), driverID, true)
	req := httptest.NewRequest(http.MethodPost, "/driver/payouts",
		bytes.NewReader([]byte(`{"amountCents":15000,"method":"bank"}`)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assertStatus(t, res.Code, http.StatusConflict, res.Body.String())
}

func TestDriverRequestWithdrawalRequiresDriverRole(t *testing.T) {
	router := newTestRouter(NewLedger(walletStoreWith(uuid.New(), 10000)), uuid.New(), false)
	req := httptest.NewRequest(http.MethodPost, "/driver/payouts",
		bytes.NewReader([]byte(`{"amountCents":15000,"method":"bank"}`)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	// gin's RequireRole returns 403 for mismatched roles.
	assertStatus(t, res.Code, http.StatusForbidden, res.Body.String())
}

func TestDriverWalletOverview(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 9500)
	store.earnings = db.GetWalletEarningsSummaryRow{
		TodayCents: "1000", WeekCents: "8600", MonthCents: "30500", TotalCents: "30500",
	}
	router := newTestRouter(NewLedger(store), driverID, true)

	req := httptest.NewRequest(http.MethodGet, "/driver/wallet", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assertStatus(t, res.Code, http.StatusOK, res.Body.String())
	body := res.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"balanceCents":9500`)) || !bytes.Contains([]byte(body), []byte(`"totalCents":30500`)) {
		t.Fatalf("wallet response = %s", body)
	}
}

func TestAdminApproveAndCompletePayout(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 10000)
	ledger := NewLedger(store)
	request, _, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 6000, Method: "multicada",
	})
	if err != nil {
		t.Fatalf("seed withdrawal: %v", err)
	}
	router := newTestRouter(ledger, driverID, false)
	payoutPath := "/admin/payouts/" + request.ID.String()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, payoutPath+"/approve", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, payoutPath+"/complete", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("complete status %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	if store.payouts[0].Status != "completed" {
		t.Fatalf("payout status %s, want completed", store.payouts[0].Status)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, payoutPath+"/complete", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("double complete status %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestAdminFailPayoutRefundsBalance(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 10000)
	request, _, err := NewLedger(store).RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 6000, Method: "multicada",
	})
	if err != nil {
		t.Fatalf("seed withdrawal: %v", err)
	}
	if _, err := NewLedger(store).ApprovePayout(context.Background(), request.ID); err != nil {
		t.Fatalf("seed approve: %v", err)
	}
	router := newTestRouter(NewLedger(store), driverID, false)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/admin/payouts/"+request.ID.String()+"/fail",
		bytes.NewReader([]byte(`{"reason":"provider failure"}`))))
	if rec.Code != http.StatusOK {
		t.Fatalf("fail status %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	if store.wallet.BalanceCents != 10000 {
		t.Fatalf("balance after fail %d, want 10000", store.wallet.BalanceCents)
	}
	if store.payouts[0].Status != "failed" {
		t.Fatalf("payout status %s, want failed", store.payouts[0].Status)
	}
}

func TestAdminPayoutQueueEmptyWhenNoProviders(t *testing.T) {
	store := walletStoreWith(uuid.New(), 10000)
	router := newTestRouter(NewLedger(store), uuid.New(), false)
	req := httptest.NewRequest(http.MethodGet, "/admin/payouts?status=processing&limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue status %d, want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"payouts":[]`)) {
		t.Fatalf("queue response: %s, want empty payouts", rec.Body)
	}
}
