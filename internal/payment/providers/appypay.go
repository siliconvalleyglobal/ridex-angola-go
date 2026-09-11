// Package providers implements payment provider adapters for RideX Angola.
// Each adapter translates between the provider-neutral payment ledger and
// a specific payment provider's API.
package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AppyPayConfig holds configuration for the AppyPay provider.
type AppyPayConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	GPONEnabled  bool
}

// AppyPayPayment represents a payment intent created via AppyPay.
type AppyPayPayment struct {
	ID               string `json:"id"`
	ProviderChargeID string `json:"providerChargeId"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	CreatedAt        string `json:"createdAt"`
	CompletedAt      string `json:"completedAt,omitempty"`
}

// AppyPayClient handles communication with the AppyPay API.
type AppyPayClient struct {
	config AppyPayConfig
	client *http.Client
	logger *zap.Logger
}

// NewAppyPayClient creates a new AppyPay client.
func NewAppyPayClient(cfg AppyPayConfig, logger *zap.Logger) *AppyPayClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://sandbox.appypay.co"
	}
	return &AppyPayClient{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// AppyPayError represents an error from the AppyPay API.
type AppyPayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppyPayError) Error() string {
	return fmt.Sprintf("AppyPay error %s: %s", e.Code, e.Message)
}

// CreatePaymentIntent creates a payment intent with AppyPay.
func (c *AppyPayClient) CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error) {
	payload := map[string]interface{}{
		"amount":   amountCents,
		"currency": currency,
		"metadata": map[string]string{
			"rideId": rideID.String(),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payment intent request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/v1/payments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("X-GPO", "true")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var appyError AppyPayError
		if err := json.NewDecoder(resp.Body).Decode(&appyError); err != nil {
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return nil, &appyError
	}

	var payment AppyPayPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Info("created AppyPay payment intent",
		zap.String("paymentId", payment.ID),
		zap.String("providerChargeId", payment.ProviderChargeID),
		zap.Int64("amountCents", amountCents),
		zap.String("currency", currency),
	)

	return &Payment{
		ID:               payment.ID,
		ProviderChargeID: payment.ProviderChargeID,
		Status:           payment.Status,
		Amount:           payment.Amount,
		Currency:         payment.Currency,
		CreatedAt:        payment.CreatedAt,
		CompletedAt:      payment.CompletedAt,
	}, nil
}

// GetPaymentStatus queries the status of a payment intent.
func (c *AppyPayClient) GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/api/v1/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s body: %s", resp.StatusCode, resp.Status, string(body))
	}

	var payment AppyPayPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Payment{
		ID:               payment.ID,
		ProviderChargeID: payment.ProviderChargeID,
		Status:           payment.Status,
		Amount:           payment.Amount,
		Currency:         payment.Currency,
		CreatedAt:        payment.CreatedAt,
		CompletedAt:      payment.CompletedAt,
	}, nil
}

// RefundPayment refunds a payment.
func (c *AppyPayClient) RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error {
	payload := map[string]interface{}{
		"amount": amountCents,
	}
	if reason != "" {
		payload["reason"] = reason
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal refund request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/v1/payments/"+paymentID+"/refund", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var appyError AppyPayError
		if err := json.NewDecoder(resp.Body).Decode(&appyError); err != nil {
			return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return &appyError
	}

	c.logger.Info("refunded AppyPay payment",
		zap.String("paymentId", paymentID),
		zap.Int64("amountCents", amountCents),
	)

	return nil
}

// authHeader returns the Authorization header value for AppyPay API requests.
func (c *AppyPayClient) authHeader() string {
	credentials := c.config.ClientID + ":" + c.config.ClientSecret
	signature := base64.StdEncoding.EncodeToString([]byte(credentials))
	return "Basic " + signature
}

// VerifyWebhookSignature verifies AppyPay webhook signatures.
// AppyPay uses HMAC-SHA256 with the client secret as the key.
func (c *AppyPayClient) VerifyWebhookSignature(payload []byte, signature string) error {
	if c.config.ClientSecret == "" {
		return errors.New("AppyPay client secret not configured")
	}

	signature = strings.TrimSpace(signature)
	if strings.HasPrefix(signature, "sha256=") {
		signature = strings.TrimPrefix(signature, "sha256=")
	}

	// Try hex decoding first (more specific), then base64
	var provided []byte
	var err error

	// Check if it looks like hex (only hex chars, even length)
	if isHexString(signature) {
		provided, err = hex.DecodeString(signature)
		if err != nil {
			return errors.New("invalid hex signature")
		}
	} else if decoded, decodeErr := base64.StdEncoding.DecodeString(signature); decodeErr == nil {
		provided = decoded
	} else {
		return errors.New("unsupported signature encoding")
	}

	mac := hmac.New(sha256.New, []byte(c.config.ClientSecret))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return errors.New("invalid webhook signature")
	}

	return nil
}

// isHexString checks if a string looks like a hex-encoded string.
func isHexString(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// AppyPayWebhookEvent represents a webhook event from AppyPay.
type AppyPayWebhookEvent struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	Data      AppyPayWebhookData `json:"data"`
	CreatedAt string             `json:"createdAt"`
	Signature string             `json:"signature,omitempty"`
}

// AppyPayWebhookData contains the payment data in a webhook event.
type AppyPayWebhookData struct {
	ID               string            `json:"id"`
	ProviderChargeID string            `json:"providerChargeId"`
	Status           string            `json:"status"`
	Amount           int64             `json:"amount"`
	Currency         string            `json:"currency"`
	Metadata         map[string]string `json:"metadata"`
}

// ParseWebhookEvent parses and validates a webhook event from AppyPay.
func (c *AppyPayClient) ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error) {
	if err := c.VerifyWebhookSignature(payload, signature); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	var event AppyPayWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode webhook event: %w", err)
	}

	_, eventType, eventPayload, err := event.ToProviderEvent()
	if err != nil {
		return nil, err
	}

	return &WebhookEvent{
		ID:               event.ID,
		Type:             eventType,
		ProviderChargeID: event.Data.ProviderChargeID,
		Data:             eventPayload,
		CreatedAt:        event.CreatedAt,
		Signature:        event.Signature,
	}, nil
}

// ToProviderEvent converts an AppyPay webhook event to a provider-neutral event.
func (e *AppyPayWebhookEvent) ToProviderEvent() (providerChargeID string, eventType string, payload []byte, err error) {
	switch e.Type {
	case "payment.created":
		eventType = "payment.processing"
	case "payment.completed":
		eventType = "payment.completed"
	case "payment.failed":
		eventType = "payment.failed"
	case "payment.refunded":
		eventType = "payment.refunded"
	default:
		return "", "", nil, fmt.Errorf("unsupported event type: %s", e.Type)
	}

	providerChargeID = e.Data.ProviderChargeID
	payload, err = json.Marshal(e.Data)
	if err != nil {
		return "", "", nil, fmt.Errorf("marshal event data: %w", err)
	}

	return providerChargeID, eventType, payload, nil
}
