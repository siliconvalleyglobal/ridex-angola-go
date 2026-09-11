package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProxyPayClient_CreatePaymentIntent(t *testing.T) {
	logger := setupTestLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if r.URL.Path != "/api/v1/payments" {
			t.Errorf("expected /api/v1/payments, got %s", r.URL.Path)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test_api_key" {
			t.Errorf("expected Bearer auth, got %s", auth)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			return
		}

		if payload["amount"] != float64(10000) {
			t.Errorf("expected amount 10000, got %v", payload["amount"])
			return
		}

		response := map[string]interface{}{
			"id":               "proxypay_pay_123",
			"providerChargeId": "proxypay_charge_456",
			"status":           "processing",
			"amount":           10000,
			"currency":         "AOA",
			"createdAt":        time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewProxyPayClient(ProxyPayConfig{
		APIKey:  "test_api_key",
		BaseURL: server.URL,
	}, logger)

	payment, err := client.CreatePaymentIntent(context.Background(), uuid.New(), 10000, "AOA")
	if err != nil {
		t.Fatalf("CreatePaymentIntent failed: %v", err)
	}

	if payment.ID != "proxypay_pay_123" {
		t.Errorf("expected payment ID proxypay_pay_123, got %s", payment.ID)
	}

	if payment.ProviderChargeID != "proxypay_charge_456" {
		t.Errorf("expected provider charge ID proxypay_charge_456, got %s", payment.ProviderChargeID)
	}
}

func TestProxyPayClient_VerifyWebhookSignature(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewProxyPayClient(ProxyPayConfig{
		APIKey:  "test_api_key",
		BaseURL: "https://api.proxypay.co",
	}, logger)

	payload := []byte(`{"id":"evt_123","type":"payment.completed"}`)

	mac := hmac.New(sha256.New, []byte("test_api_key"))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if err := client.VerifyWebhookSignature(payload, expectedSig); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}

	if err := client.VerifyWebhookSignature(payload, "invalid_signature"); err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestProxyPayClient_ParseWebhookEvent(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewProxyPayClient(ProxyPayConfig{
		APIKey:  "test_api_key",
		BaseURL: "https://api.proxypay.co",
	}, logger)

	payload := []byte(`{
		"id": "evt_123",
		"type": "payment.completed",
		"data": {
			"id": "pay_456",
			"providerChargeId": "charge_789",
			"status": "completed",
			"amount": 10000,
			"currency": "AOA",
			"reference": "ride_001"
		},
		"createdAt": "2024-01-01T00:00:00Z"
	}`)

	mac := hmac.New(sha256.New, []byte("test_api_key"))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	event, err := client.ParseWebhookEvent(payload, signature)
	if err != nil {
		t.Fatalf("ParseWebhookEvent failed: %v", err)
	}

	if event.ID != "evt_123" {
		t.Errorf("expected event ID evt_123, got %s", event.ID)
	}

	if event.Type != "payment.completed" {
		t.Errorf("expected event type payment.completed, got %s", event.Type)
	}
}
