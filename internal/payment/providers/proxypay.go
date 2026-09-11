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

// ProxyPayConfig holds configuration for the ProxyPay provider.
type ProxyPayConfig struct {
	APIKey  string
	BaseURL string
}

// ProxyPayClient handles communication with the ProxyPay API.
type ProxyPayClient struct {
	config ProxyPayConfig
	client *http.Client
	logger *zap.Logger
}

// NewProxyPayClient creates a new ProxyPay client.
func NewProxyPayClient(cfg ProxyPayConfig, logger *zap.Logger) *ProxyPayClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.proxypay.co"
	}
	return &ProxyPayClient{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ProxyPayError represents an error from the ProxyPay API.
type ProxyPayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ProxyPayError) Error() string {
	return fmt.Sprintf("ProxyPay error %s: %s", e.Code, e.Message)
}

// CreatePaymentIntent creates a payment intent with ProxyPay.
func (c *ProxyPayClient) CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error) {
	payload := map[string]interface{}{
		"amount":      amountCents,
		"currency":    currency,
		"reference":   rideID.String(),
		"description": fmt.Sprintf("RideX Angola ride %s", rideID.String()),
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
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var proxyError ProxyPayError
		if err := json.NewDecoder(resp.Body).Decode(&proxyError); err != nil {
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return nil, &proxyError
	}

	var payment ProxyPayPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Info("created ProxyPay payment intent",
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
func (c *ProxyPayClient) GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/api/v1/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s body: %s", resp.StatusCode, resp.Status, string(body))
	}

	var payment ProxyPayPayment
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

// VerifyWebhookSignature verifies ProxyPay webhook signatures.
// ProxyPay uses HMAC-SHA256 with the API key as the key.
func (c *ProxyPayClient) VerifyWebhookSignature(payload []byte, signature string) error {
	if c.config.APIKey == "" {
		return errors.New("ProxyPay API key not configured")
	}

	signature = strings.TrimSpace(signature)
	if strings.HasPrefix(signature, "sha256=") {
		signature = strings.TrimPrefix(signature, "sha256=")
	}

	var provided []byte
	if decoded, err := hex.DecodeString(signature); err == nil {
		provided = decoded
	} else if decoded, err := base64.StdEncoding.DecodeString(signature); err == nil {
		provided = decoded
	} else {
		return errors.New("unsupported signature encoding")
	}

	mac := hmac.New(sha256.New, []byte(c.config.APIKey))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return errors.New("invalid webhook signature")
	}

	return nil
}

// ProxyPayWebhookEvent represents a webhook event from ProxyPay.
type ProxyPayWebhookEvent struct {
	ID        string              `json:"id"`
	Type      string              `json:"type"`
	Data      ProxyPayWebhookData `json:"data"`
	CreatedAt string              `json:"createdAt"`
	Signature string              `json:"signature,omitempty"`
}

// ProxyPayWebhookData contains the payment data in a webhook event.
type ProxyPayWebhookData struct {
	ID               string `json:"id"`
	ProviderChargeID string `json:"providerChargeId"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	Reference        string `json:"reference"`
}

// ParseWebhookEvent parses and validates a webhook event from ProxyPay.
func (c *ProxyPayClient) ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error) {
	if err := c.VerifyWebhookSignature(payload, signature); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	var event ProxyPayWebhookEvent
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

// ToProviderEvent converts a ProxyPay webhook event to a provider-neutral event.
func (e *ProxyPayWebhookEvent) ToProviderEvent() (providerChargeID string, eventType string, payload []byte, err error) {
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

// ProxyPayPayment is an internal type for ProxyPay payment responses.
type ProxyPayPayment struct {
	ID               string `json:"id"`
	ProviderChargeID string `json:"providerChargeId"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	CreatedAt        string `json:"createdAt"`
	CompletedAt      string `json:"completedAt,omitempty"`
}

// RefundPayment refunds a payment.
func (c *ProxyPayClient) RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error {
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
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var proxyError ProxyPayError
		if err := json.NewDecoder(resp.Body).Decode(&proxyError); err != nil {
			return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return &proxyError
	}

	c.logger.Info("refunded ProxyPay payment",
		zap.String("paymentId", paymentID),
		zap.Int64("amountCents", amountCents),
	)

	return nil
}
