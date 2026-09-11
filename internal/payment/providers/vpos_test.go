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

func TestVPOSClient_CreatePaymentIntent(t *testing.T) {
	logger := setupTestLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if r.URL.Path != "/v1/payments" {
			t.Errorf("expected /v1/payments, got %s", r.URL.Path)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
			return
		}

		if r.Header.Get("X-Developer-ID") != "test_dev" {
			t.Errorf("expected X-Developer-ID test_dev")
			return
		}

		if r.Header.Get("X-API-Key") != "test_api_key" {
			t.Errorf("expected X-API-Key test_api_key")
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
			"id":               "vpos_pay_123",
			"providerChargeId": "vpos_charge_456",
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

	client := NewVPOSClient(VPOSConfig{
		DeveloperID:   "test_dev",
		APIKey:        "test_api_key",
		BaseURL:       server.URL,
		WebhookSecret: "test_webhook_secret",
	}, logger)

	payment, err := client.CreatePaymentIntent(context.Background(), uuid.New(), 10000, "AOA")
	if err != nil {
		t.Fatalf("CreatePaymentIntent failed: %v", err)
	}

	if payment.ID != "vpos_pay_123" {
		t.Errorf("expected payment ID vpos_pay_123, got %s", payment.ID)
	}

	if payment.ProviderChargeID != "vpos_charge_456" {
		t.Errorf("expected provider charge ID vpos_charge_456, got %s", payment.ProviderChargeID)
	}
}

func TestVPOSClient_VerifyWebhookSignature(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewVPOSClient(VPOSConfig{
		DeveloperID:   "test_dev",
		APIKey:        "test_api_key",
		BaseURL:       "https://api.vpos.ao",
		WebhookSecret: "test_webhook_secret",
	}, logger)

	payload := []byte(`{"id":"evt_123","type":"payment.completed"}`)

	mac := hmac.New(sha256.New, []byte("test_webhook_secret"))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if err := client.VerifyWebhookSignature(payload, expectedSig); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}

	if err := client.VerifyWebhookSignature(payload, "invalid_signature"); err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestVPOSClient_ParseWebhookEvent(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewVPOSClient(VPOSConfig{
		DeveloperID:   "test_dev",
		APIKey:        "test_api_key",
		BaseURL:       "https://api.vpos.ao",
		WebhookSecret: "test_webhook_secret",
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

	mac := hmac.New(sha256.New, []byte("test_webhook_secret"))
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

func TestVPOSClient_SignRequest(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewVPOSClient(VPOSConfig{
		DeveloperID:   "test_dev",
		APIKey:        "test_api_key",
		BaseURL:       "https://api.vpos.ao",
		WebhookSecret: "test_webhook_secret",
	}, logger)

	body := []byte(`{"amount":10000}`)
	signature := client.signRequest(body, "test_dev", "test_api_key")

	mac := hmac.New(sha256.New, []byte("test_api_key"))
	message := "test_dev" + string(body) + "test_api_key"
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if signature != expectedSig {
		t.Errorf("expected signature %s, got %s", expectedSig, signature)
	}
}
