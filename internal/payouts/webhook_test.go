package payouts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const testWebhookSecret = "payout-webhook-secret-32-bytes!!"

func signBody(t *testing.T, body []byte, secret string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"reference":"x","status":"completed"}`)
	sig := signBody(t, body, testWebhookSecret)
	if err := VerifyWebhookSignature(body, sig, testWebhookSecret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyWebhookSignature(body, "sha256="+sig, testWebhookSecret); err != nil {
		t.Fatalf("sha256= prefix rejected: %v", err)
	}
	if err := VerifyWebhookSignature(body, sig, "another-secret"); !errors.Is(err, ErrInvalidWebhook) {
		t.Fatalf("wrong secret err = %v, want ErrInvalidWebhook", err)
	}
	if err := VerifyWebhookSignature(body, "not-hex!!", testWebhookSecret); !errors.Is(err, ErrInvalidWebhook) {
		t.Fatalf("malformed err = %v, want ErrInvalidWebhook", err)
	}
	if err := VerifyWebhookSignature(body, sig, "  "); !errors.Is(err, ErrWebhookNotConfigured) {
		t.Fatalf("unset secret err = %v, want ErrWebhookNotConfigured", err)
	}
}

func TestRecordPayoutCallbackSettlesCompleted(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	if _, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: request.ID, Status: StatusCompleted}); err != nil {
		t.Fatalf("completed callback: %v", err)
	}
	if store.payouts[0].Status != "completed" || store.wallet.BalanceCents != 4000 || store.lastPayoutCalls != 1 {
		t.Fatalf("settlement = status %q balance %d lastPayout %d", store.payouts[0].Status, store.wallet.BalanceCents, store.lastPayoutCalls)
	}
}

func TestRecordPayoutCallbackReversesAndRejectsLateCompletion(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	// Processing callback: accepted no-op.
	still, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: request.ID, Status: StatusProcessing})
	if err != nil || still.Status != "processing" {
		t.Fatalf("processing callback = %q, %v; want no-op", still.Status, err)
	}
	// Failed callback: reversal with an auditable reason.
	if _, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: request.ID, Status: StatusFailed, Message: "account closed"}); err != nil {
		t.Fatalf("failed callback: %v", err)
	}
	if store.wallet.BalanceCents != 10000 {
		t.Fatalf("balance = %d, want 10000 refunded", store.wallet.BalanceCents)
	}
	if !strings.Contains(store.payouts[0].FailureReason.String, "provider callback: account closed") {
		t.Fatalf("failure reason = %q", store.payouts[0].FailureReason.String)
	}
	// A late completion after reversal is a transition conflict, never applied.
	if _, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: request.ID, Status: StatusCompleted}); !errors.Is(err, ErrPayoutTransition) {
		t.Fatalf("late completed err = %v, want ErrPayoutTransition", err)
	}
}

func TestRecordPayoutCallbackRejectsUnknownStatusAndMissingPayout(t *testing.T) {
	ledger := NewLedger(walletStoreWith(uuid.New(), 10000))
	if _, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: uuid.New(), Status: "mystery"}); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("unknown status err = %v, want ErrInvalidCallback", err)
	}
	if _, err := ledger.RecordPayoutCallback(context.Background(), PayoutCallback{PayoutID: uuid.New(), Status: StatusProcessing}); !errors.Is(err, ErrPayoutNotFound) {
		t.Fatalf("missing payout err = %v, want ErrPayoutNotFound", err)
	}
}

func TestPayoutWebhookEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	handler := NewHandler(ledger).WithWebhookSecret(testWebhookSecret)
	router := gin.New()
	router.POST("/payouts/webhooks/:executor", handler.PayoutWebhook)

	do := func(body []byte, sig string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/payouts/webhooks/bank", bytes.NewReader(body))
		if sig != "" {
			req.Header.Set("X-Payout-Signature", sig)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	payload := []byte(`{"reference":"` + request.ID.String() + `","status":"completed"}`)
	// Unsigned: fails closed.
	if rec := do(payload, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned = %d, want 401", rec.Code)
	}
	// Signed: settles.
	if rec := do(payload, signBody(t, payload, testWebhookSecret)); rec.Code != http.StatusAccepted {
		t.Fatalf("signed = %d body %s, want 202", rec.Code, rec.Body.String())
	}
	if store.payouts[0].Status != "completed" {
		t.Fatalf("payout status = %q, want completed", store.payouts[0].Status)
	}
	// Duplicate callback: idempotent conflict, money untouched.
	if rec := do(payload, signBody(t, payload, testWebhookSecret)); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate = %d, want 409", rec.Code)
	}
	if store.wallet.BalanceCents != 4000 {
		t.Fatalf("balance = %d, want 4000 after a single settlement", store.wallet.BalanceCents)
	}
	// Bad reference shape: 400.
	bad := []byte(`{"reference":"not-a-uuid","status":"completed"}`)
	if rec := do(bad, signBody(t, bad, testWebhookSecret)); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad reference = %d, want 400", rec.Code)
	}
}
