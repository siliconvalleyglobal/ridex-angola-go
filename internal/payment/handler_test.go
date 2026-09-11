package payment

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/payment/providers"
)

func TestRefundHandlerMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()

	setup := func(store *fakeStore, refunder Refunder) *gin.Engine {
		handler := NewHandler(store, nil).WithRefunder(refunder)
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(auth.ContextUserID, adminID.String()); c.Next() })
		router.POST("/admin/payments/:paymentId/refund", handler.Refund)
		return router
	}

	do := func(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	// Completed charge refunds successfully.
	store := &fakeStore{charge: completedCharge()}
	router := setup(store, &fakeRefunder{})
	res := do(router, "/admin/payments/"+store.charge.ID.String()+"/refund", `{"reason":"rider overcharged"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("refund status = %d, want %d: %s", res.Code, http.StatusOK, res.Body)
	}
	if !strings.Contains(res.Body.String(), `"refunded":true`) {
		t.Fatalf("refund response = %s, want refunded true", res.Body)
	}
	if len(store.events) != 1 || store.events[0].EventType != "payment.refunded" {
		t.Fatalf("refund ledger events = %#v, want one payment.refunded", store.events)
	}

	// Pending charge is a conflict.
	pending := &fakeStore{charge: completedCharge()}
	pending.charge.Status = "pending"
	res = do(setup(pending, &fakeRefunder{}), "/admin/payments/"+pending.charge.ID.String()+"/refund", `{}`)
	if res.Code != http.StatusConflict {
		t.Fatalf("pending refund status = %d, want %d", res.Code, http.StatusConflict)
	}

	// Unknown charge is 404.
	missing := &fakeStore{charge: completedCharge()}
	res = do(setup(missing, &fakeRefunder{}), "/admin/payments/"+uuid.New().String()+"/refund", `{}`)
	if res.Code != http.StatusNotFound {
		t.Fatalf("unknown refund status = %d, want %d", res.Code, http.StatusNotFound)
	}

	// No configured refunder is 503.
	unavailable := &fakeStore{charge: completedCharge()}
	res = do(setup(unavailable, nil), "/admin/payments/"+unavailable.charge.ID.String()+"/refund", `{}`)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured refunder status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	if len(unavailable.events) != 0 {
		t.Fatalf("unconfigured refunder recorded events")
	}
}

func TestReconcileHandlerReturnsCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	store := &fakeStore{pending: []db.PaymentCharge{pendingCharge("vpos", "VPOS-1")}}
	handler := NewHandler(store, nil).WithStatusPoller(&fakePoller{provider: "vpos", statuses: map[string]IntentStatus{
		"VPOS-1": {ProviderChargeID: "VPOS-1", Status: "completed"},
	}})
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(auth.ContextUserID, adminID.String()); c.Next() })
	router.POST("/admin/reconciliation/payments", handler.Reconcile)

	req := httptest.NewRequest(http.MethodPost, "/admin/reconciliation/payments?limit=5", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d, want %d: %s", res.Code, http.StatusOK, res.Body)
	}
	if !strings.Contains(res.Body.String(), `"updated":1`) {
		t.Fatalf("reconcile response = %s, want updated 1", res.Body)
	}

	// Without a poller the trigger reports unavailable instead of failing.
	empty := &fakeStore{}
	handler = NewHandler(empty, nil)
	router = gin.New()
	router.Use(func(c *gin.Context) { c.Set(auth.ContextUserID, adminID.String()); c.Next() })
	router.POST("/admin/reconciliation/payments", handler.Reconcile)
	req = httptest.NewRequest(http.MethodPost, "/admin/reconciliation/payments", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured reconcile status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestWebhookHandlerVerifiesBeforeRecording(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeStore{charge: dbCharge("intent_1")}
	verifier := NewHMACSHA256Verifier("secret")
	handler := NewHandler(store, map[string]WebhookVerifier{"vpos": verifier})
	router := gin.New()
	router.POST("/payments/webhooks/:provider/:providerChargeID", handler.Webhook)

	payload := `{"status":"completed"}`
	req := httptest.NewRequest(http.MethodPost, "/payments/webhooks/vpos/intent_1", strings.NewReader(payload))
	req.Header.Set("X-Payment-Event-ID", "evt-1")
	req.Header.Set("X-Payment-Event-Type", "payment.completed")
	req.Header.Set("X-Payment-Signature", verifier.Sign([]byte(payload)))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("valid webhook status = %d, want %d: %s", res.Code, http.StatusAccepted, res.Body)
	}
	if len(store.events) != 1 {
		t.Fatalf("recorded events = %d, want one", len(store.events))
	}

	req = httptest.NewRequest(http.MethodPost, "/payments/webhooks/vpos/intent_1", strings.NewReader("tampered"))
	req.Header.Set("X-Payment-Event-ID", "evt-2")
	req.Header.Set("X-Payment-Event-Type", "payment.completed")
	req.Header.Set("X-Payment-Signature", verifier.Sign([]byte(payload)))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("invalid webhook status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if len(store.events) != 1 {
		t.Fatalf("invalid webhook was recorded")
	}
}

func dbCharge(providerChargeID string) db.PaymentCharge {
	return db.PaymentCharge{ID: uuid.New(), Provider: "vpos", ProviderChargeID: providerChargeID, Status: "pending"}
}

type fakeAdapter struct {
	evt *providers.WebhookEvent
	err error
}

func (f *fakeAdapter) ParseWebhookEvent([]byte, string) (*providers.WebhookEvent, error) {
	return f.evt, f.err
}

func TestWebhookAdapterTranslatesNativePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeStore{charge: dbCharge("APPY-9")}
	handler := NewHandler(store, map[string]WebhookVerifier{
		"appypay": NewHMACSHA256Verifier("neutral-secret"),
	}).WithAdapters(map[string]ProviderWebhookAdapter{
		"appypay": &fakeAdapter{evt: &providers.WebhookEvent{
			ID: "evt-9", Type: "payment.completed", ProviderChargeID: "APPY-9",
		}},
	})
	router := gin.New()
	router.POST("/payments/webhooks/:provider/:providerChargeID", handler.Webhook)

	req := httptest.NewRequest(http.MethodPost, "/payments/webhooks/appypay/APPY-9",
		strings.NewReader(`{"type":"payment.completed","data":{"id":"APPY-9"}}`))
	req.Header.Set("X-Payment-Signature", "provider-signature")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("adapter webhook status = %d, want %d: %s", res.Code, http.StatusAccepted, res.Body)
	}
	if len(store.events) != 1 {
		t.Fatalf("events recorded = %d, want one", len(store.events))
	}
	if store.events[0].EventType != "payment.completed" || store.events[0].ProviderEventID.String != "evt-9" {
		t.Fatalf("canonical event = %#v, want completed/evt-9", store.events[0])
	}
	if store.charge.Status != "completed" {
		t.Fatalf("charge status = %q, want completed", store.charge.Status)
	}
}

func TestWebhookAdapterRejectsUnverifiedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeStore{charge: dbCharge("APPY-9")}
	handler := NewHandler(store, map[string]WebhookVerifier{
		"appypay": NewHMACSHA256Verifier("neutral-secret"),
	}).WithAdapters(map[string]ProviderWebhookAdapter{
		"appypay": &fakeAdapter{err: errors.New("invalid webhook signature")},
	})
	router := gin.New()
	router.POST("/payments/webhooks/:provider/:providerChargeID", handler.Webhook)

	req := httptest.NewRequest(http.MethodPost, "/payments/webhooks/appypay/APPY-9",
		strings.NewReader(`{"type":"payment.completed"}`))
	req.Header.Set("X-Payment-Signature", "tampered")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("tampered adapter webhook status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if len(store.events) != 0 {
		t.Fatalf("rejected adapter webhook was recorded")
	}
}

func TestWebhookAdapterRequiresCanonicalMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeStore{charge: dbCharge("APPY-9")}
	handler := NewHandler(store, map[string]WebhookVerifier{
		"appypay": NewHMACSHA256Verifier("neutral-secret"),
	}).WithAdapters(map[string]ProviderWebhookAdapter{
		"appypay": &fakeAdapter{evt: &providers.WebhookEvent{ID: "evt-9", Type: "payment.completed"}},
	})
	router := gin.New()
	router.POST("/payments/webhooks/:provider/:providerChargeID", handler.Webhook)

	req := httptest.NewRequest(http.MethodPost, "/payments/webhooks/appypay/APPY-9",
		strings.NewReader(`{}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("missing charge id status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if len(store.events) != 0 {
		t.Fatalf("event recorded without provider charge id")
	}
}
