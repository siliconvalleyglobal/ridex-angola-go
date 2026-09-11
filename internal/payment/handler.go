package payment

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/payment/providers"
)

const maxWebhookBody = 1 << 20

// ProviderWebhookAdapter translates a provider's native webhook payload into
// the canonical ledger event. Implementations must verify the provider's own
// signature scheme before parsing; the returned event is trusted as verified.
type ProviderWebhookAdapter interface {
	ParseWebhookEvent(payload []byte, signature string) (*providers.WebhookEvent, error)
}

// passthroughVerifier accepts payloads whose signature was already verified
// by a provider adapter against the provider's own secret.
type passthroughVerifier struct{}

func (passthroughVerifier) Verify([]byte, string) error { return nil }

// Handler exposes provider-neutral payment ledger endpoints. Provider
// adapters can translate their documented webhook into the headers and raw
// body expected here without coupling the ride domain to that provider.
type Handler struct {
	service   *Service
	verifiers map[string]WebhookVerifier
	adapters  map[string]ProviderWebhookAdapter
}

func NewHandler(store Store, verifiers map[string]WebhookVerifier) *Handler {
	copied := make(map[string]WebhookVerifier, len(verifiers))
	for provider, verifier := range verifiers {
		copied[strings.ToLower(strings.TrimSpace(provider))] = verifier
	}
	return &Handler{service: NewService(store), verifiers: copied}
}

// NewHandlerWithCharger wires an optional provider charger into the payment
// service so intent creation reserves the charge at the configured provider.
func NewHandlerWithCharger(store Store, verifiers map[string]WebhookVerifier, charger Charger) *Handler {
	handler := NewHandler(store, verifiers)
	handler.service.WithCharger(charger)
	return handler
}

// WithAdapters registers provider webhook adapters so native provider
// payloads are verified and translated before reaching the ledger.
func (h *Handler) WithAdapters(adapters map[string]ProviderWebhookAdapter) *Handler {
	copied := make(map[string]ProviderWebhookAdapter, len(adapters))
	for provider, adapter := range adapters {
		copied[strings.ToLower(strings.TrimSpace(provider))] = adapter
	}
	h.adapters = copied
	return h
}

// WithRefunder wires an optional provider refund boundary into the payment
// service so admin refund requests execute at the configured provider.
func (h *Handler) WithRefunder(refunder Refunder) *Handler {
	h.service.WithRefunder(refunder)
	return h
}

// WithStatusPoller wires an optional provider status boundary into the
// payment service so reconciliation can resolve missed webhooks.
func (h *Handler) WithStatusPoller(poller StatusPoller) *Handler {
	h.service.WithStatusPoller(poller)
	return h
}

type refundRequest struct {
	Reason string `json:"reason"`
}

// Refund refunds a completed payment at the configured provider and records
// the audited ledger event. Admin role is enforced at the route level; the
// acting admin's id is persisted on the refund event for audit.
func (h *Handler) Refund(c *gin.Context) {
	adminID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	chargeID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req refundRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refund request body"})
		return
	}
	charge, event, err := h.service.RefundCharge(c.Request.Context(), RefundInput{
		ChargeID: chargeID, ActorID: adminID, Reason: req.Reason,
	})
	switch {
	case errors.Is(err, ErrRefunderUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refund provider is not configured"})
	case errors.Is(err, ErrPaymentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "payment charge not found"})
	case errors.Is(err, ErrRefundNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"error": "only completed payments can be refunded"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refund input"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to refund payment"})
	default:
		c.JSON(http.StatusOK, gin.H{"payment": PaymentJSON(charge), "event": EventJSON(event), "refunded": true})
	}
}

// Reconcile runs one reconciliation pass over pending/processing charges
// using the configured provider status poller.
func (h *Handler) Reconcile(c *gin.Context) {
	if _, err := uuid.Parse(c.GetString(auth.ContextUserID)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 0
	}
	result, err := h.service.ReconcilePending(c.Request.Context(), limit)
	switch {
	case errors.Is(err, ErrPollerUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment status polling is not configured"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reconcile payments"})
	default:
		c.JSON(http.StatusOK, gin.H{"reconciliation": result})
	}
}

type createIntentRequest struct {
	RideID         string `json:"rideId" binding:"required"`
	Provider       string `json:"provider" binding:"required"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *Handler) CreateIntent(c *gin.Context) {
	riderID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req createIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rideId and provider are required"})
		return
	}
	rideID, err := uuid.Parse(req.RideID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ride id"})
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		key = strings.TrimSpace(req.IdempotencyKey)
	}
	charge, replay, err := h.service.CreateIntentWithReplay(c.Request.Context(), CreateIntentInput{
		RiderID: riderID, RideID: rideID, Provider: req.Provider, IdempotencyKey: key,
	})
	switch {
	case errors.Is(err, ErrPaymentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
	case errors.Is(err, ErrPaymentForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "not your ride"})
	case errors.Is(err, ErrUnknownProvider), errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrFraudVelocityExceeded), errors.Is(err, ErrFraudDuplicateAmount):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment intent"})
	default:
		status := http.StatusCreated
		if replay {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"paymentIntent": PaymentJSON(charge), "idempotentReplay": replay})
	}
}

// Webhook accepts an opaque provider payload. The charge identifier is taken
// from the path when present, or X-Payment-Charge-ID for adapter integrations.
// X-Payment-Event-ID and X-Payment-Event-Type are canonical ledger metadata;
// they are not a claim about any provider's native payload format.
//
// When a provider adapter is registered for the path's provider, the native
// payload is verified against the provider's own secret and translated into
// the canonical event (charge ID, event ID, canonical type) before reaching
// the ledger; the neutral metadata headers are then ignored.
func (h *Handler) Webhook(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	if !AllowedProvider(provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported payment provider"})
		return
	}
	providerChargeID := strings.TrimSpace(c.Param("providerChargeID"))
	if providerChargeID == "" {
		providerChargeID = strings.TrimSpace(c.GetHeader("X-Payment-Charge-ID"))
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBody))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "webhook body is too large"})
		return
	}
	signature := c.GetHeader("X-Payment-Signature")

	verifier := h.verifiers[provider]
	in := WebhookInput{
		Provider: provider, ProviderChargeID: providerChargeID,
		ProviderEventID: c.GetHeader("X-Payment-Event-ID"),
		EventType:       c.GetHeader("X-Payment-Event-Type"),
		Payload:         body, Signature: signature,
	}
	if adapter := h.adapters[provider]; adapter != nil {
		evt, parseErr := adapter.ParseWebhookEvent(body, signature)
		if parseErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
			return
		}
		if evt.Type == "" || evt.ProviderChargeID == "" || evt.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment event metadata"})
			return
		}
		in = WebhookInput{
			Provider: provider, ProviderChargeID: evt.ProviderChargeID,
			ProviderEventID: evt.ID, EventType: evt.Type,
			Payload: body, Signature: signature,
		}
		// The adapter already verified the provider's own signature scheme.
		verifier = passthroughVerifier{}
	}
	charge, ledgerEvent, err := h.service.RecordWebhook(c.Request.Context(), verifier, in)
	switch {
	case errors.Is(err, ErrVerifierUnavailable), errors.Is(err, ErrInvalidSignature), errors.Is(err, ErrUnsupportedSignature):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
	case errors.Is(err, ErrInvalidEvent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment event metadata"})
	case errors.Is(err, ErrPaymentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "payment charge not found"})
	case errors.Is(err, ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrStatusTransition):
		c.JSON(http.StatusConflict, gin.H{"error": "payment status transition is not allowed"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record payment event"})
	default:
		c.JSON(http.StatusAccepted, gin.H{
			"payment": PaymentJSON(charge), "event": EventJSON(ledgerEvent), "recorded": true,
		})
	}
}
