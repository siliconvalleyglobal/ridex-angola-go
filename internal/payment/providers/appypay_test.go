package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func setupTestLogger(t *testing.T) *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	return logger
}

func TestAppyPayClient_CreatePaymentIntent(t *testing.T) {
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
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Basic auth header")
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
			"id":               "pay_test_123",
			"providerChargeId": "appypay_charge_456",
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

	client := NewAppyPayClient(AppyPayConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		BaseURL:      server.URL,
		GPONEnabled:  true,
	}, logger)

	payment, err := client.CreatePaymentIntent(context.Background(), uuid.New(), 10000, "AOA")
	if err != nil {
		t.Fatalf("CreatePaymentIntent failed: %v", err)
	}

	if payment.ID != "pay_test_123" {
		t.Errorf("expected payment ID pay_test_123, got %s", payment.ID)
	}

	if payment.ProviderChargeID != "appypay_charge_456" {
		t.Errorf("expected provider charge ID appypay_charge_456, got %s", payment.ProviderChargeID)
	}

	if payment.Status != "processing" {
		t.Errorf("expected status processing, got %s", payment.Status)
	}
}

func TestAppyPayClient_VerifyWebhookSignature(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewAppyPayClient(AppyPayConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		BaseURL:      "https://sandbox.appypay.co",
	}, logger)

	payload := []byte(`{"id":"evt_123","type":"payment.completed"}`)

	mac := hmac.New(sha256.New, []byte("test_secret"))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if err := client.VerifyWebhookSignature(payload, "sha256="+expectedSig); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}

	// Also test with raw hex (without sha256= prefix)
	if err := client.VerifyWebhookSignature(payload, expectedSig); err != nil {
		t.Errorf("expected valid signature without prefix, got error: %v", err)
	}

	invalidSig := "invalid_signature"
	if err := client.VerifyWebhookSignature(payload, invalidSig); err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestAppyPayClient_ParseWebhookEvent(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewAppyPayClient(AppyPayConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		BaseURL:      "https://sandbox.appypay.co",
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
			"metadata": {
				"rideId": "ride_001"
			}
		},
		"createdAt": "2024-01-01T00:00:00Z"
	}`)

	mac := hmac.New(sha256.New, []byte("test_secret"))
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

func TestAppyPayClient_AuthHeader(t *testing.T) {
	logger := setupTestLogger(t)
	client := NewAppyPayClient(AppyPayConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		BaseURL:      "https://sandbox.appypay.co",
	}, logger)

	authHeader := client.authHeader()

	if !strings.HasPrefix(authHeader, "Basic ") {
		t.Error("expected Basic auth header")
		return
	}

	encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
	credentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		t.Fatalf("failed to decode credentials: %v", err)
	}

	expectedCredentials := "test_client:test_secret"
	if string(credentials) != expectedCredentials {
		t.Errorf("expected credentials %s, got %s", expectedCredentials, string(credentials))
	}
}
