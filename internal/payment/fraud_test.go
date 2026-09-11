package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ridex/ridex-angola/internal/db"
)

// fakeFraudStore implements FraudStore and records the probe arguments for
// assertions. Its fields are intentionally simple integers so a test table
// can drive every code path without a real database.
type fakeFraudStore struct {
	signals  db.GetRiderChargeSignalsRow
	err      error
	lastArgs db.GetRiderChargeSignalsParams
	called   bool
}

func (f *fakeFraudStore) GetRiderChargeSignals(_ context.Context, arg db.GetRiderChargeSignalsParams) (db.GetRiderChargeSignalsRow, error) {
	f.called = true
	f.lastArgs = arg
	return f.signals, f.err
}

func fraudTestConfig() FraudConfig {
	return FraudConfig{
		VelocityWindow:  time.Hour,
		VelocityLimit:   10,
		DuplicateWindow: 30 * time.Minute,
		DuplicateLimit:  2,
	}
}

func mustFraud(t *testing.T, store *fakeFraudStore) *Fraud {
	t.Helper()
	fraud, err := NewFraud(store, fraudTestConfig())
	if err != nil {
		t.Fatalf("NewFraud: %v", err)
	}
	return fraud
}

func TestNewFraudRejectsNilStore(t *testing.T) {
	if _, err := NewFraud(nil, fraudTestConfig()); err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestNewFraudRejectsInvalidConfig(t *testing.T) {
	store := &fakeFraudStore{}
	for _, tt := range []struct {
		name string
		cfg  FraudConfig
	}{
		{"negative velocity window", FraudConfig{VelocityWindow: -1, VelocityLimit: 10,
			DuplicateWindow: 30 * time.Minute, DuplicateLimit: 2}},
		{"zero velocity limit", FraudConfig{VelocityWindow: time.Hour, VelocityLimit: 0,
			DuplicateWindow: 30 * time.Minute, DuplicateLimit: 2}},
		{"negative duplicate window", FraudConfig{VelocityWindow: time.Hour, VelocityLimit: 10,
			DuplicateWindow: -1, DuplicateLimit: 2}},
		{"zero duplicate limit", FraudConfig{VelocityWindow: time.Hour, VelocityLimit: 10,
			DuplicateWindow: 30 * time.Minute, DuplicateLimit: 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewFraud(store, tt.cfg); err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestEvaluateAllowsWhenSignalsBelowLimits(t *testing.T) {
	store := &fakeFraudStore{signals: db.GetRiderChargeSignalsRow{RecentCharges: 3, DuplicateAmounts: 1}}
	f := mustFraud(t, store)
	sigs, err := f.Evaluate(context.Background(), uuid.New(), 25000)
	if err != nil {
		t.Fatalf("Evaluate: unexpected error %v", err)
	}
	if sigs.RecentCharges != 3 || sigs.DuplicateAmounts != 1 {
		t.Fatalf("signals = %+v, want {3, 1}", sigs)
	}
	if !store.called {
		t.Fatal("expected store.GetRiderChargeSignals to be called")
	}
}

func TestEvaluateVelocityLimitInclusive(t *testing.T) {
	store := &fakeFraudStore{signals: db.GetRiderChargeSignalsRow{
		RecentCharges: fraudTestConfig().VelocityLimit,
	}}
	f := mustFraud(t, store)
	_, err := f.Evaluate(context.Background(), uuid.New(), 25000)
	if !errors.Is(err, ErrFraudVelocityExceeded) {
		t.Fatalf("Evaluate: got %v, want ErrFraudVelocityExceeded", err)
	}
}

func TestEvaluateDuplicateLimitInclusive(t *testing.T) {
	store := &fakeFraudStore{signals: db.GetRiderChargeSignalsRow{
		DuplicateAmounts: fraudTestConfig().DuplicateLimit,
	}}
	f := mustFraud(t, store)
	_, err := f.Evaluate(context.Background(), uuid.New(), 25000)
	if !errors.Is(err, ErrFraudDuplicateAmount) {
		t.Fatalf("Evaluate: got %v, want ErrFraudDuplicateAmount", err)
	}
}

func TestEvaluateFailsClosedOnStoreError(t *testing.T) {
	store := &fakeFraudStore{err: errors.New("database unavailable")}
	f := mustFraud(t, store)
	if _, err := f.Evaluate(context.Background(), uuid.New(), 25000); err == nil {
		t.Fatal("Evaluate: expected error on store failure, got nil")
	}
}

func TestEvaluatePassesCorrectArgs(t *testing.T) {
	store := &fakeFraudStore{signals: db.GetRiderChargeSignalsRow{RecentCharges: 0, DuplicateAmounts: 0}}
	f := mustFraud(t, store)
	riderID := uuid.New()
	_, _ = f.Evaluate(context.Background(), riderID, 37500)
	args := store.lastArgs
	if args.RiderID != riderID {
		t.Errorf("RiderID = %v, want %v", args.RiderID, riderID)
	}
	// $2 (outer scan bound) = $3 (velocity window) = now - 1hr
	// $4 (duplicate window) = now - 30min
	// allow a few ms skew since time.Now() is called once in Evaluate and again here.
	now := time.Now().UTC()
	wantVelocity := now.Add(-f.config.VelocityWindow)   // 1hr ago
	wantDuplicate := now.Add(-f.config.DuplicateWindow) // 30min ago
	if args.CreatedAt.Time.Sub(wantVelocity).Abs() > 5*time.Millisecond {
		t.Errorf("CreatedAt (outer $2) = %v, want ~%v", args.CreatedAt.Time, wantVelocity)
	}
	if args.CreatedAt_2.Time.Sub(wantVelocity).Abs() > 5*time.Millisecond {
		t.Errorf("CreatedAt_2 (velocity $3) = %v, want ~%v", args.CreatedAt_2.Time, wantVelocity)
	}
	if args.CreatedAt_3.Time.Sub(wantDuplicate).Abs() > 5*time.Millisecond {
		t.Errorf("CreatedAt_3 (duplicate $4) = %v, want ~%v", args.CreatedAt_3.Time, wantDuplicate)
	}
	if !args.CreatedAt_3.Time.After(args.CreatedAt_2.Time) {
		t.Error("CreatedAt_3 (duplicate, 30min ago) should be after CreatedAt_2 (velocity, 1hr ago)")
	}
}

// replayStore embeds fakeStore and adds the idempotency lookup that the
// service probes with a type assertion, so the fraud-gate path can be
// exercised through the real Service.CreateIntent call site.
type replayStore struct {
	*fakeStore
	existing db.PaymentCharge
}

func (s *replayStore) GetPaymentChargeByIdempotencyKey(context.Context, db.GetPaymentChargeByIdempotencyKeyParams) (db.PaymentCharge, error) {
	return s.existing, nil
}

func TestServiceCreateIntentBlocksOnVelocity(t *testing.T) {
	fake := &fakeFraudStore{signals: db.GetRiderChargeSignalsRow{
		RecentCharges: fraudTestConfig().VelocityLimit,
	}}
	riderID, rideID := uuid.New(), uuid.New()
	store := &fakeStore{ride: db.Ride{
		ID: rideID, RiderID: riderID, PaymentMethod: "card", Currency: "AOA",
		SuggestedFareCents: NumericFromInt64(2500),
	}}
	svc := NewService(store).WithFraud(mustFraud(t, fake))
	_, err := svc.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: riderID, RideID: rideID, Provider: "vpos", IdempotencyKey: "velocity-block",
	})
	if !errors.Is(err, ErrFraudVelocityExceeded) {
		t.Fatalf("CreateIntent: got %v, want ErrFraudVelocityExceeded", err)
	}
	if !fake.called {
		t.Error("expected fraud probe to be called")
	}
}

func TestServiceCreateIntentNotEvaluatedOnReplay(t *testing.T) {
	fake := &fakeFraudStore{}
	riderID, rideID := uuid.New(), uuid.New()
	ride := db.Ride{
		ID: rideID, RiderID: riderID, PaymentMethod: "card", Currency: "AOA",
		SuggestedFareCents: NumericFromInt64(2500),
	}
	charge := db.PaymentCharge{
		ID: uuid.New(), RideID: rideID, Provider: "vpos", Currency: "AOA",
		ProviderChargeID: "intent_abc", AmountCents: NumericFromInt64(2500),
	}
	rs := &replayStore{
		fakeStore: &fakeStore{ride: ride},
		existing:  charge,
	}
	svc := NewService(rs).WithFraud(mustFraud(t, fake))
	got, err := svc.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: riderID, RideID: rideID, Provider: "vpos", IdempotencyKey: "replay",
	})
	if err != nil {
		t.Fatalf("CreateIntent: unexpected error %v", err)
	}
	if got.ID != charge.ID {
		t.Errorf("charge ID = %v, want %v", got.ID, charge.ID)
	}
	if fake.called {
		t.Error("fraud must not be evaluated on idempotency replay")
	}
}
