package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

var (
	ErrInvalidInput        = errors.New("invalid payment input")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrPaymentForbidden    = errors.New("payment is not owned by the caller")
	ErrIdempotencyConflict = errors.New("idempotency key was already used with different payment details")
	ErrUnknownProvider     = errors.New("unsupported payment provider")
	ErrInvalidEvent        = errors.New("invalid payment event")
	ErrStatusTransition    = errors.New("payment status transition is not allowed")
)

// Store is the database surface used by the provider-neutral payment ledger.
// Keeping it narrow makes the domain safe to exercise without a live provider
// or database.
type Store interface {
	GetRideByID(context.Context, uuid.UUID) (db.Ride, error)
	CreatePaymentIntent(context.Context, db.CreatePaymentIntentParams) (db.PaymentCharge, error)
	GetPaymentChargeByProviderChargeID(context.Context, db.GetPaymentChargeByProviderChargeIDParams) (db.PaymentCharge, error)
	GetPaymentChargeByID(context.Context, uuid.UUID) (db.PaymentCharge, error)
	GetPendingCharges(context.Context, int32) ([]db.PaymentCharge, error)
	RecordPaymentEvent(context.Context, db.RecordPaymentEventParams) (db.PaymentEvent, error)
	ApplyPaymentEventStatus(context.Context, db.ApplyPaymentEventStatusParams) (db.PaymentCharge, error)
}

type intentLookup interface {
	GetPaymentChargeByIdempotencyKey(context.Context, db.GetPaymentChargeByIdempotencyKeyParams) (db.PaymentCharge, error)
}

// Service creates local payment intents and records signed webhook events.
// It deliberately does not know a provider payload format. When a Charger is
// attached, intent creation additionally reserves the charge at the configured
// provider and persists the provider-issued charge identifier.
type Service struct {
	store    Store
	charger  Charger
	refunder Refunder
	poller   StatusPoller
}

func NewService(store Store) *Service { return &Service{store: store} }

// WithCharger attaches an optional provider intent boundary. A nil charger
// keeps ledger-only behavior.
func (s *Service) WithCharger(charger Charger) *Service {
	s.charger = charger
	return s
}

// WithRefunder attaches an optional provider refund boundary. A nil refunder
// keeps refund attempts rejected with ErrRefunderUnavailable instead of
// silently marking charges refunded without a provider call.
func (s *Service) WithRefunder(refunder Refunder) *Service {
	s.refunder = refunder
	return s
}

// WithStatusPoller attaches an optional provider status boundary used by the
// reconciliation job to resolve charges whose webhooks were missed.
func (s *Service) WithStatusPoller(poller StatusPoller) *Service {
	s.poller = poller
	return s
}

type CreateIntentInput struct {
	RiderID        uuid.UUID
	RideID         uuid.UUID
	Provider       string
	IdempotencyKey string
}

func (s *Service) CreateIntent(ctx context.Context, in CreateIntentInput) (db.PaymentCharge, error) {
	charge, _, err := s.createIntent(ctx, in)
	return charge, err
}

// CreateIntentWithReplay is the retry-aware variant used by HTTP handlers.
// Existing stores that support the generated lookup query return the original
// charge without generating a new opaque intent identifier.
func (s *Service) CreateIntentWithReplay(ctx context.Context, in CreateIntentInput) (db.PaymentCharge, bool, error) {
	return s.createIntent(ctx, in)
}

func (s *Service) createIntent(ctx context.Context, in CreateIntentInput) (db.PaymentCharge, bool, error) {
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.RiderID == uuid.Nil || in.RideID == uuid.Nil || in.IdempotencyKey == "" {
		return db.PaymentCharge{}, false, ErrInvalidInput
	}
	if !AllowedProvider(in.Provider) {
		return db.PaymentCharge{}, false, ErrUnknownProvider
	}
	if len(in.IdempotencyKey) > 200 {
		return db.PaymentCharge{}, false, ErrInvalidInput
	}

	ride, err := s.store.GetRideByID(ctx, in.RideID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PaymentCharge{}, false, ErrPaymentNotFound
	}
	if err != nil {
		return db.PaymentCharge{}, false, fmt.Errorf("load ride: %w", err)
	}
	if ride.RiderID != in.RiderID {
		return db.PaymentCharge{}, false, ErrPaymentForbidden
	}
	if ride.PaymentMethod == "cash" {
		return db.PaymentCharge{}, false, fmt.Errorf("%w: cash rides do not require an online intent", ErrInvalidInput)
	}
	if ride.Status == "cancelled" {
		return db.PaymentCharge{}, false, fmt.Errorf("%w: cancelled rides cannot be paid", ErrInvalidInput)
	}

	amount := ride.SuggestedFareCents
	if ride.AcceptedFareCents.Valid {
		amount = ride.AcceptedFareCents
	}
	if !amount.Valid || amount.Int == nil || amount.Int.Sign() <= 0 || amount.Exp != 0 {
		return db.PaymentCharge{}, false, fmt.Errorf("%w: ride has no valid fare", ErrInvalidInput)
	}

	if lookup, ok := s.store.(intentLookup); ok {
		existing, lookupErr := lookup.GetPaymentChargeByIdempotencyKey(ctx, db.GetPaymentChargeByIdempotencyKeyParams{
			RideID: in.RideID, IdempotencyKey: pgtype.Text{String: in.IdempotencyKey, Valid: true},
		})
		if lookupErr == nil {
			if existing.Provider != in.Provider || existing.Currency != ride.Currency ||
				!numericEqual(existing.AmountCents, amount) {
				return db.PaymentCharge{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return db.PaymentCharge{}, false, fmt.Errorf("load payment intent idempotency key: %w", lookupErr)
		}
	}

	// Without a charger the ledger runs in local-only mode and payment_charges
	// keeps an opaque local identifier that must not be sent to any provider
	// as a provider-generated ID. With a charger, the intent is reserved at
	// the provider first and the provider-issued identifier is persisted; a
	// provider failure leaves nothing stored so the request can be retried.
	providerChargeID := ""
	if s.charger != nil {
		result, err := s.charger.CreatePaymentIntent(ctx, in.RideID, amount.Int.Int64(), ride.Currency)
		if err != nil {
			return db.PaymentCharge{}, false, fmt.Errorf("create provider intent: %w", err)
		}
		if result == nil || result.ProviderChargeID == "" {
			return db.PaymentCharge{}, false, fmt.Errorf("provider returned no charge identifier")
		}
		providerChargeID = result.ProviderChargeID
	} else {
		providerChargeID = "intent_" + uuid.NewString()
	}
	charge, err := s.store.CreatePaymentIntent(ctx, db.CreatePaymentIntentParams{
		RideID:           in.RideID,
		Provider:         in.Provider,
		ProviderChargeID: providerChargeID,
		IdempotencyKey:   pgtype.Text{String: in.IdempotencyKey, Valid: true},
		AmountCents:      amount,
		Currency:         ride.Currency,
	})
	if err != nil {
		return db.PaymentCharge{}, false, fmt.Errorf("create payment intent: %w", err)
	}
	if charge.RideID != in.RideID || charge.Provider != in.Provider ||
		charge.Currency != ride.Currency || !numericEqual(charge.AmountCents, amount) {
		return db.PaymentCharge{}, false, ErrIdempotencyConflict
	}
	return charge, false, nil
}

type WebhookInput struct {
	Provider         string
	ProviderChargeID string
	ProviderEventID  string
	EventType        string
	Payload          []byte
	Signature        string
}

func (s *Service) RecordWebhook(ctx context.Context, verifier WebhookVerifier, in WebhookInput) (db.PaymentCharge, db.PaymentEvent, error) {
	if verifier == nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrVerifierUnavailable
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.ProviderChargeID = strings.TrimSpace(in.ProviderChargeID)
	in.ProviderEventID = strings.TrimSpace(in.ProviderEventID)
	in.EventType = strings.TrimSpace(in.EventType)
	if !AllowedProvider(in.Provider) || in.ProviderChargeID == "" ||
		in.ProviderEventID == "" || in.EventType == "" || len(in.ProviderEventID) > 200 ||
		len(in.EventType) > 120 || len(in.Payload) == 0 {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrInvalidEvent
	}
	if err := verifier.Verify(in.Payload, in.Signature); err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, err
	}

	charge, err := s.store.GetPaymentChargeByProviderChargeID(ctx, db.GetPaymentChargeByProviderChargeIDParams{
		Provider: in.Provider, ProviderChargeID: in.ProviderChargeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrPaymentNotFound
	}
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("load payment charge: %w", err)
	}

	event, err := s.store.RecordPaymentEvent(ctx, db.RecordPaymentEventParams{
		ChargeID:        charge.ID,
		EventType:       in.EventType,
		Payload:         append([]byte(nil), in.Payload...),
		Signature:       pgtype.Text{String: in.Signature, Valid: in.Signature != ""},
		ProviderEventID: pgtype.Text{String: in.ProviderEventID, Valid: true},
	})
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("record payment event: %w", err)
	}
	if event.EventType != in.EventType || !bytes.Equal(event.Payload, in.Payload) {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrIdempotencyConflict
	}

	status, ok := canonicalStatus(in.EventType)
	if !ok {
		return charge, event, nil
	}
	updated, err := s.store.ApplyPaymentEventStatus(ctx, db.ApplyPaymentEventStatusParams{
		ID: charge.ID, Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrStatusTransition
	}
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("apply payment event: %w", err)
	}
	return updated, event, nil
}

func AllowedProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "appypay", "vpos", "proxypay":
		return true
	default:
		return false
	}
}

func canonicalStatus(eventType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "payment.processing":
		return "processing", true
	case "payment.completed":
		return "completed", true
	case "payment.failed":
		return "failed", true
	case "payment.refunded":
		return "refunded", true
	default:
		return "", false
	}
}

func numericEqual(a, b pgtype.Numeric) bool {
	if !a.Valid || !b.Valid || a.Int == nil || b.Int == nil {
		return a.Valid == b.Valid && a.Int == nil && b.Int == nil
	}
	return a.Exp == b.Exp && a.Int.Cmp(b.Int) == 0
}

func NumericFromInt64(value int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(value), Exp: 0, Valid: true}
}

func PaymentJSON(charge db.PaymentCharge) map[string]interface{} {
	return map[string]interface{}{
		"id":               charge.ID,
		"rideId":           charge.RideID,
		"provider":         charge.Provider,
		"providerChargeId": charge.ProviderChargeID,
		"idempotencyKey":   charge.IdempotencyKey,
		"amountCents":      charge.AmountCents,
		"currency":         charge.Currency,
		"status":           charge.Status,
		"createdAt":        charge.CreatedAt,
		"updatedAt":        charge.UpdatedAt,
		"completedAt":      charge.CompletedAt,
		"refundedAt":       charge.RefundedAt,
	}
}

func EventJSON(event db.PaymentEvent) map[string]interface{} {
	var payload interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		payload = string(event.Payload)
	}
	return map[string]interface{}{
		"id":              event.ID,
		"chargeId":        event.ChargeID,
		"eventType":       event.EventType,
		"providerEventId": event.ProviderEventID,
		"signature":       event.Signature,
		"payload":         payload,
		"receivedAt":      event.ReceivedAt,
	}
}
