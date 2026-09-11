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

// VPOSConfig holds configuration for the VPOS provider.
type VPOSConfig struct {
	DeveloperID   string
	APIKey        string
	BaseURL       string
	WebhookSecret string
}

// VPOSClient handles communication with the VPOS API.
type VPOSClient struct {
	config VPOSConfig
	client *http.Client
	logger *zap.Logger
}

// NewVPOSClient creates a new VPOS client.
func NewVPOSClient(cfg VPOSConfig, logger *zap.Logger) *VPOSClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.vpos.ao"
	}
	return &VPOSClient{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// VPOSError represents an error from the VPOS API.
type VPOSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *VPOSError) Error() string {
	return fmt.Sprintf("VPOS error %s: %s", e.Code, e.Message)
}

// CreatePaymentIntent creates a payment intent with VPOS.
func (c *VPOSClient) CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/payments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Developer-ID", c.config.DeveloperID)
	req.Header.Set("X-API-Key", c.config.APIKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	// Add HMAC signature for request authentication
	signature := c.signRequest(body, c.config.DeveloperID, c.config.APIKey)
	req.Header.Set("X-Signature", signature)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var vposError VPOSError
		if err := json.NewDecoder(resp.Body).Decode(&vposError); err != nil {
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return nil, &vposError
	}

	var payment VPOSPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Info("created VPOS payment intent",
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
func (c *VPOSClient) GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/v1/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Developer-ID", c.config.DeveloperID)
	req.Header.Set("X-API-Key", c.config.APIKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s body: %s", resp.StatusCode, resp.Status, string(body))
	}

	var payment VPOSPayment
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
func (c *VPOSClient) RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/payments/"+paymentID+"/refund", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Developer-ID", c.config.DeveloperID)
	req.Header.Set("X-API-Key", c.config.APIKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	signature := c.signRequest(body, c.config.DeveloperID, c.config.APIKey)
	req.Header.Set("X-Signature", signature)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var vposError VPOSError
		if err := json.NewDecoder(resp.Body).Decode(&vposError); err != nil {
			return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, resp.Status)
		}
		return &vposError
	}

	c.logger.Info("refunded VPOS payment",
		zap.String("paymentId", paymentID),
		zap.Int64("amountCents", amountCents),
	)

	return nil
}

// signRequest creates an HMAC-SHA256 signature for VPOS API requests.
func (c *VPOSClient) signRequest(body []byte, developerID, apiKey string) string {
	message := developerID + string(body) + apiKey
	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies VPOS webhook signatures.
// VPOS uses HMAC-SHA256 with the webhook secret as the key.
func (c *VPOSClient) VerifyWebhookSignature(payload []byte, signature string) error {
	if c.config.WebhookSecret == "" {
		return errors.New("VPOS webhook secret not configured")
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

	mac := hmac.New(sha256.New, []byte(c.config.WebhookSecret))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return errors.New("invalid webhook signature")
	}

	return nil
}

// VPOSWebhookEvent represents a webhook event from VPOS.
type VPOSWebhookEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Data      VPOSWebhookData `json:"data"`
	CreatedAt string          `json:"createdAt"`
	Signature string          `json:"signature,omitempty"`
}

// VPOSWebhookData contains the payment data in a webhook event.
type VPOSWebhookData struct {
	ID               string `json:"id"`
	ProviderChargeID string `json:"providerChargeId"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	Reference        string `json:"reference"`
}

// ParseWebhookEvent parses and validates a webhook event from VPOS.
func (c *VPOSClient) ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error) {
	if err := c.VerifyWebhookSignature(payload, signature); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	var event VPOSWebhookEvent
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

// ToProviderEvent converts a VPOS webhook event to a provider-neutral event.
func (e *VPOSWebhookEvent) ToProviderEvent() (providerChargeID string, eventType string, payload []byte, err error) {
	switch e.Type {
	case "payment.created", "payment.initialized":
		eventType = "payment.processing"
	case "payment.completed", "payment.success":
		eventType = "payment.completed"
	case "payment.failed", "payment.cancelled":
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

// VPOSPayment is an internal type for VPOS payment responses.
type VPOSPayment struct {
	ID               string `json:"id"`
	ProviderChargeID string `json:"providerChargeId"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	CreatedAt        string `json:"createdAt"`
	CompletedAt      string `json:"completedAt,omitempty"`
}
