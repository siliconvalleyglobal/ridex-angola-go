package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/payment/providers"
)

var (
	// ErrRefunderUnavailable means no provider refund boundary is configured,
	// so a refund can never be executed for real. The ledger is left intact.
	ErrRefunderUnavailable = errors.New("refund provider is not configured")
	// ErrRefundNotAllowed means the charge is not in a state that can be
	// refunded; only completed charges are refundable.
	ErrRefundNotAllowed = errors.New("only completed payments can be refunded")
)

// maxRefundReason bounds the audit reason persisted in the ledger event.
const maxRefundReason = 500

// Refunder is the optional boundary for executing refunds at a configured
// provider. Like Charger it keeps the ledger independent of any provider's
// request or response format.
type Refunder interface {
	RefundPayment(ctx context.Context, providerChargeID string, amountCents int64, reason string) error
}

// ProviderRefunder adapts a providers.Provider implementation to the Refunder
// boundary.
type ProviderRefunder struct {
	Provider providers.Provider
}

// RefundPayment delegates to the provider adapter.
func (r ProviderRefunder) RefundPayment(ctx context.Context, providerChargeID string, amountCents int64, reason string) error {
	if r.Provider == nil {
		return ErrRefunderUnavailable
	}
	return r.Provider.RefundPayment(ctx, providerChargeID, amountCents, reason)
}

// RefundInput describes an administrator-initiated refund.
type RefundInput struct {
	ChargeID uuid.UUID
	ActorID  uuid.UUID // admin user requesting the refund, kept for audit
	Reason   string
}

// RefundCharge executes a full refund of a completed charge at the configured
// provider and records a payment.refunded ledger event before applying the
// guarded status transition. Refunds are full-charge only: the schema tracks a
// single refunded status per charge, so partial refunds are out of scope until
// a refunded_amount column exists.
func (s *Service) RefundCharge(ctx context.Context, in RefundInput) (db.PaymentCharge, db.PaymentEvent, error) {
	if s.refunder == nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrRefunderUnavailable
	}
	if in.ChargeID == uuid.Nil || in.ActorID == uuid.Nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrInvalidInput
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Reason) > maxRefundReason {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrInvalidInput
	}

	charge, err := s.store.GetPaymentChargeByID(ctx, in.ChargeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrPaymentNotFound
	}
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("load payment charge: %w", err)
	}
	if charge.Status != "completed" {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrRefundNotAllowed
	}
	if !charge.AmountCents.Valid || charge.AmountCents.Int == nil ||
		charge.AmountCents.Int.Sign() <= 0 || charge.AmountCents.Exp != 0 {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("%w: charge has no refundable amount", ErrInvalidInput)
	}
	amount := charge.AmountCents.Int.Int64()

	if err := s.refunder.RefundPayment(ctx, charge.ProviderChargeID, amount, in.Reason); err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("provider refund: %w", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"source":      "admin",
		"reason":      in.Reason,
		"actorId":     in.ActorID,
		"amountCents": amount,
	})
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("marshal refund event: %w", err)
	}
	event, err := s.store.RecordPaymentEvent(ctx, db.RecordPaymentEventParams{
		ChargeID:  charge.ID,
		EventType: "payment.refunded",
		Payload:   payload,
	})
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("record refund event: %w", err)
	}
	if event.EventType != "payment.refunded" || !bytes.Equal(event.Payload, payload) {
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrIdempotencyConflict
	}

	updated, err := s.store.ApplyPaymentEventStatus(ctx, db.ApplyPaymentEventStatusParams{
		ID: charge.ID, Status: "refunded",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent webhook moved the charge into a non-refundable state
		// after the provider accepted the refund. Surface it: the event is on
		// the ledger and the reconciliation report will flag the divergence.
		return db.PaymentCharge{}, db.PaymentEvent{}, ErrRefundNotAllowed
	}
	if err != nil {
		return db.PaymentCharge{}, db.PaymentEvent{}, fmt.Errorf("apply refund status: %w", err)
	}
	return updated, event, nil
}
