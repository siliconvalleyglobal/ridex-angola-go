package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/db"
)

type fakeRefunder struct {
	calls        int
	lastChargeID string
	lastAmount   int64
	lastReason   string
	err          error
}

func (r *fakeRefunder) RefundPayment(_ context.Context, providerChargeID string, amountCents int64, reason string) error {
	r.calls++
	r.lastChargeID = providerChargeID
	r.lastAmount = amountCents
	r.lastReason = reason
	return r.err
}

func completedCharge() db.PaymentCharge {
	return db.PaymentCharge{
		ID: uuid.New(), RideID: uuid.New(), Provider: "vpos", ProviderChargeID: "VPOS-7",
		AmountCents: NumericFromInt64(1800), Currency: "AOA", Status: "completed",
	}
}

func TestRefundChargeRequiresConfiguredRefunder(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	service := NewService(store)

	_, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.New(), Reason: "duplicate charge",
	})
	if !errors.Is(err, ErrRefunderUnavailable) {
		t.Fatalf("refund without refunder: err = %v, want ErrRefunderUnavailable", err)
	}
	if len(store.events) != 0 || store.charge.Status != "completed" {
		t.Fatalf("ledger changed without a refunder: events=%d status=%s", len(store.events), store.charge.Status)
	}
}

func TestRefundChargeRequiresCompletedCharge(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	store.charge.Status = "pending"
	refunder := &fakeRefunder{}
	service := NewService(store).WithRefunder(refunder)

	_, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.New(), Reason: "wrong state",
	})
	if !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("refund pending charge: err = %v, want ErrRefundNotAllowed", err)
	}
	if refunder.calls != 0 {
		t.Fatalf("provider refund called %d times for non-completed charge", refunder.calls)
	}
}

func TestRefundChargeNotFound(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	service := NewService(store).WithRefunder(&fakeRefunder{})

	_, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: uuid.New(), ActorID: uuid.New(), Reason: "missing",
	})
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("refund unknown charge: err = %v, want ErrPaymentNotFound", err)
	}
}

func TestRefundChargeValidatesActorAndReason(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	service := NewService(store).WithRefunder(&fakeRefunder{})

	if _, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.Nil, Reason: "no actor",
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("refund without actor: err = %v, want ErrInvalidInput", err)
	}
	long := strings.Repeat("x", maxRefundReason+1)
	if _, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.New(), Reason: long,
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("refund with oversized reason: err = %v, want ErrInvalidInput", err)
	}
}

func TestRefundChargeExecutesProviderRefundAndRecordsLedger(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	actorID := uuid.New()
	refunder := &fakeRefunder{}
	service := NewService(store).WithRefunder(refunder)

	charge, event, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: actorID, Reason: " rider cancelled after charge ",
	})
	if err != nil {
		t.Fatalf("refund charge: %v", err)
	}
	if refunder.calls != 1 || refunder.lastChargeID != "VPOS-7" || refunder.lastAmount != 1800 {
		t.Fatalf("provider refund call = %dx %s/%d, want 1x VPOS-7/1800", refunder.calls, refunder.lastChargeID, refunder.lastAmount)
	}
	if refunder.lastReason != "rider cancelled after charge" {
		t.Fatalf("provider refund reason = %q, want trimmed reason", refunder.lastReason)
	}
	if charge.Status != "refunded" {
		t.Fatalf("charge status = %q, want refunded", charge.Status)
	}
	if event.EventType != "payment.refunded" || event.ProviderEventID.Valid {
		t.Fatalf("refund event = %#v, want payment.refunded without provider event id", event)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("refund event payload: %v", err)
	}
	if payload["source"] != "admin" || payload["actorId"] != actorID.String() || payload["reason"] != "rider cancelled after charge" {
		t.Fatalf("refund audit payload = %#v", payload)
	}
	if len(store.events) != 1 {
		t.Fatalf("recorded events = %d, want one", len(store.events))
	}
}

func TestRefundChargeProviderFailureLeavesLedgerUntouched(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	service := NewService(store).WithRefunder(&fakeRefunder{err: context.DeadlineExceeded})

	_, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.New(), Reason: "provider down",
	})
	if err == nil {
		t.Fatal("expected provider failure to fail the refund")
	}
	if len(store.events) != 0 || store.charge.Status != "completed" {
		t.Fatalf("ledger changed despite provider failure: events=%d status=%s", len(store.events), store.charge.Status)
	}
}

func TestRefundChargeRaceSurfacesTransitionConflict(t *testing.T) {
	store := &fakeStore{charge: completedCharge()}
	// Simulate a concurrent webhook failing the charge while the provider
	// refund call is in flight: the guarded transition must surface the
	// conflict instead of silently overwriting the failed state.
	refunder := &racingRefunder{store: store}
	service := NewService(store).WithRefunder(refunder)

	_, _, err := service.RefundCharge(context.Background(), RefundInput{
		ChargeID: store.charge.ID, ActorID: uuid.New(), Reason: "race",
	})
	if !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("refund race: err = %v, want ErrRefundNotAllowed", err)
	}
	if store.charge.Status != "failed" {
		t.Fatalf("charge status = %q, want failed state preserved", store.charge.Status)
	}
}

// racingRefunder flips the stored charge to a non-refundable state during the
// provider call, emulating a webhook that lands between load and apply.
type racingRefunder struct{ store *fakeStore }

func (r *racingRefunder) RefundPayment(context.Context, string, int64, string) error {
	r.store.charge.Status = "failed"
	return nil
}
