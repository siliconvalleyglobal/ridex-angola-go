package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/db"
)

type fakePoller struct {
	provider string
	statuses map[string]IntentStatus
	errs     map[string]error
	calls    []string
}

func (p *fakePoller) Provider() string { return p.provider }

func (p *fakePoller) GetPaymentStatus(_ context.Context, providerChargeID string) (IntentStatus, error) {
	p.calls = append(p.calls, providerChargeID)
	if err := p.errs[providerChargeID]; err != nil {
		return IntentStatus{}, err
	}
	return p.statuses[providerChargeID], nil
}

func pendingCharge(provider, providerChargeID string) db.PaymentCharge {
	return db.PaymentCharge{
		ID: uuid.New(), RideID: uuid.New(), Provider: provider,
		ProviderChargeID: providerChargeID, AmountCents: NumericFromInt64(2500),
		Currency: "AOA", Status: "pending",
	}
}

func TestReconcilePendingRequiresPoller(t *testing.T) {
	store := &fakeStore{pending: []db.PaymentCharge{pendingCharge("vpos", "VPOS-1")}}
	service := NewService(store)

	if _, err := service.ReconcilePending(context.Background(), 0); !errors.Is(err, ErrPollerUnavailable) {
		t.Fatalf("reconcile without poller: err = %v, want ErrPollerUnavailable", err)
	}
}

func TestReconcilePendingAppliesMissedWebhook(t *testing.T) {
	charge := pendingCharge("vpos", "VPOS-1")
	store := &fakeStore{pending: []db.PaymentCharge{charge}}
	poller := &fakePoller{provider: "vpos", statuses: map[string]IntentStatus{
		"VPOS-1": {ProviderChargeID: "VPOS-1", Status: "completed"},
	}}
	service := NewService(store).WithStatusPoller(poller)

	result, err := service.ReconcilePending(context.Background(), 0)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.Checked != 1 || result.Updated != 1 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("reconcile result = %#v, want checked/updated = 1", result)
	}
	events := store.events
	if len(events) != 1 {
		t.Fatalf("recorded events = %d, want one", len(events))
	}
	if events[0].EventType != "payment.completed" {
		t.Fatalf("event type = %q, want payment.completed", events[0].EventType)
	}
	if events[0].ProviderEventID.String != "recon:VPOS-1:completed" || !events[0].ProviderEventID.Valid {
		t.Fatalf("reconcile event id = %#v, want deterministic recon id", events[0].ProviderEventID)
	}
	if string(events[0].Payload) != `{"source":"reconciliation","status":"completed"}` {
		t.Fatalf("reconcile payload = %s, want deterministic reconciliation payload", events[0].Payload)
	}

	// Second pass: the charge is no longer pending, so nothing is polled.
	result, err = service.ReconcilePending(context.Background(), 0)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if result.Checked != 0 || result.Updated != 0 {
		t.Fatalf("second reconcile result = %#v, want nothing to do", result)
	}
	if len(poller.calls) != 1 {
		t.Fatalf("poller calls = %d, want 1", len(poller.calls))
	}
}

func TestReconcilePendingSkipsForeignProvidersAndAmbiguousStatuses(t *testing.T) {
	foreign := pendingCharge("appypay", "APPY-1")
	sameStatus := pendingCharge("vpos", "VPOS-2")
	ambiguous := pendingCharge("vpos", "VPOS-3")
	broken := pendingCharge("vpos", "VPOS-4")
	store := &fakeStore{pending: []db.PaymentCharge{foreign, sameStatus, ambiguous, broken}}
	poller := &fakePoller{provider: "vpos",
		statuses: map[string]IntentStatus{
			"VPOS-2": {ProviderChargeID: "VPOS-2", Status: "pending"},
			"VPOS-3": {ProviderChargeID: "VPOS-3", Status: "awaiting_customer"},
		},
		errs: map[string]error{"VPOS-4": context.DeadlineExceeded},
	}
	service := NewService(store).WithStatusPoller(poller)

	result, err := service.ReconcilePending(context.Background(), 10)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.Skipped != 3 || result.Failed != 1 || result.Updated != 0 {
		t.Fatalf("reconcile result = %#v, want 3 skipped / 1 failed / 0 updated", result)
	}
	if len(store.events) != 0 {
		t.Fatalf("events recorded = %d, want none", len(store.events))
	}
	if len(poller.calls) != 3 {
		t.Fatalf("poller calls = %d, want all vpos charges polled once", len(poller.calls))
	}
}

func TestReconcilePendingClampsLimit(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store).WithStatusPoller(&fakePoller{provider: "vpos"})

	if _, err := service.ReconcilePending(context.Background(), 0); err != nil {
		t.Fatalf("reconcile default limit: %v", err)
	}
	if store.lastLimit != reconcileDefaultLimit {
		t.Fatalf("limit = %d, want default %d", store.lastLimit, reconcileDefaultLimit)
	}
	if _, err := service.ReconcilePending(context.Background(), 500); err != nil {
		t.Fatalf("reconcile oversized limit: %v", err)
	}
	if store.lastLimit != reconcileMaxLimit {
		t.Fatalf("limit = %d, want clamped %d", store.lastLimit, reconcileMaxLimit)
	}
}

func TestCanonicalProviderStatusMapping(t *testing.T) {
	cases := map[string]string{
		"completed": "completed", "PAID": "completed", "succeeded": "completed",
		"processing": "processing", "IN_PROGRESS": "processing",
		"failed": "failed", "declined": "failed",
		"refunded": "refunded",
		"created":  "", "awaiting_payment": "", "": "", "  ": "",
	}
	for raw, want := range cases {
		if got := canonicalProviderStatus(raw); got != want {
			t.Fatalf("canonicalProviderStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}
