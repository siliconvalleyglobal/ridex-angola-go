package payment

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

type fakeStore struct {
	ride          db.Ride
	charge        db.PaymentCharge
	pending       []db.PaymentCharge
	events        []db.PaymentEvent
	createdIntent db.CreatePaymentIntentParams
	lastLimit     int32
}

func (f *fakeStore) GetRideByID(context.Context, uuid.UUID) (db.Ride, error) {
	return f.ride, nil
}

func (f *fakeStore) GetPaymentChargeByID(_ context.Context, id uuid.UUID) (db.PaymentCharge, error) {
	if f.charge.ID == id {
		return f.charge, nil
	}
	return db.PaymentCharge{}, pgx.ErrNoRows
}

func (f *fakeStore) GetPendingCharges(_ context.Context, limit int32) ([]db.PaymentCharge, error) {
	f.lastLimit = limit
	var pending []db.PaymentCharge
	for _, charge := range f.pending {
		if charge.Status == "pending" || charge.Status == "processing" {
			pending = append(pending, charge)
		}
	}
	if int32(len(pending)) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

func (f *fakeStore) CreatePaymentIntent(_ context.Context, arg db.CreatePaymentIntentParams) (db.PaymentCharge, error) {
	f.createdIntent = arg
	if f.charge.ID == uuid.Nil {
		f.charge.ID = uuid.New()
	}
	f.charge.RideID = arg.RideID
	f.charge.Provider = arg.Provider
	f.charge.ProviderChargeID = arg.ProviderChargeID
	f.charge.Currency = arg.Currency
	f.charge.AmountCents = arg.AmountCents
	f.charge.Status = "pending"
	return f.charge, nil
}

func (f *fakeStore) GetPaymentChargeByProviderChargeID(context.Context, db.GetPaymentChargeByProviderChargeIDParams) (db.PaymentCharge, error) {
	return f.charge, nil
}

func (f *fakeStore) RecordPaymentEvent(_ context.Context, arg db.RecordPaymentEventParams) (db.PaymentEvent, error) {
	event := db.PaymentEvent{
		ID: uuid.New(), ChargeID: arg.ChargeID, EventType: arg.EventType,
		Payload: arg.Payload, Signature: arg.Signature, ProviderEventID: arg.ProviderEventID,
	}
	for _, existing := range f.events {
		if existing.ChargeID == event.ChargeID && existing.ProviderEventID == event.ProviderEventID {
			return existing, nil
		}
	}
	f.events = append(f.events, event)
	return event, nil
}

func (f *fakeStore) ApplyPaymentEventStatus(_ context.Context, arg db.ApplyPaymentEventStatusParams) (db.PaymentCharge, error) {
	allowed := map[string]bool{
		"pending->processing": true, "processing->processing": true,
		"pending->completed": true, "processing->completed": true, "completed->completed": true,
		"pending->failed": true, "processing->failed": true, "failed->failed": true,
		"completed->refunded": true, "refunded->refunded": true,
	}
	target := &f.charge
	if arg.ID != f.charge.ID {
		found := false
		for i := range f.pending {
			if f.pending[i].ID == arg.ID {
				target = &f.pending[i]
				found = true
				break
			}
		}
		if !found {
			return db.PaymentCharge{}, pgx.ErrNoRows
		}
	}
	if !allowed[target.Status+"->"+arg.Status] {
		return db.PaymentCharge{}, pgx.ErrNoRows
	}
	target.Status = arg.Status
	return *target, nil
}

func TestCreateIntentUsesRideFareAndIdempotencyKey(t *testing.T) {
	riderID, rideID := uuid.New(), uuid.New()
	store := &fakeStore{ride: db.Ride{
		ID: rideID, RiderID: riderID, PaymentMethod: "card", Currency: "AOA",
		SuggestedFareCents: NumericFromInt64(1800),
	}}
	service := NewService(store)

	charge, err := service.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: riderID, RideID: rideID, Provider: "vpos", IdempotencyKey: "checkout-1",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if charge.Status != "pending" || charge.AmountCents.Int.Int64() != 1800 {
		t.Fatalf("charge = %#v, want pending AOA 1800", charge)
	}
	if store.createdIntent.IdempotencyKey.String != "checkout-1" || !store.createdIntent.IdempotencyKey.Valid {
		t.Fatalf("idempotency key = %#v", store.createdIntent.IdempotencyKey)
	}
}

func TestRecordWebhookIsIdempotentAndRejectsPayloadReuse(t *testing.T) {
	chargeID := uuid.New()
	store := &fakeStore{charge: db.PaymentCharge{
		ID: chargeID, Provider: "vpos", ProviderChargeID: "intent_1", Status: "pending",
	}}
	service := NewService(store)
	verifier := NewHMACSHA256Verifier("secret")
	payload := []byte(`{"status":"completed"}`)
	input := WebhookInput{
		Provider: "vpos", ProviderChargeID: "intent_1", ProviderEventID: "evt-1",
		EventType: "payment.completed", Payload: payload, Signature: verifier.Sign(payload),
	}

	if _, _, err := service.RecordWebhook(context.Background(), verifier, input); err != nil {
		t.Fatalf("record first webhook: %v", err)
	}
	if _, _, err := service.RecordWebhook(context.Background(), verifier, input); err != nil {
		t.Fatalf("record duplicate webhook: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("events recorded = %d, want one idempotent event", len(store.events))
	}

	otherPayload := []byte(`{"status":"failed"}`)
	input.Payload = otherPayload
	input.Signature = verifier.Sign(otherPayload)
	if _, _, err := service.RecordWebhook(context.Background(), verifier, input); err != ErrIdempotencyConflict {
		t.Fatalf("payload reuse error = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestCreateIntentRejectsCashRide(t *testing.T) {
	store := &fakeStore{ride: db.Ride{
		ID: uuid.New(), RiderID: uuid.New(), PaymentMethod: "cash",
		Currency: "AOA", SuggestedFareCents: pgtype.Numeric{Int: NumericFromInt64(100).Int, Valid: true},
	}}
	_, err := NewService(store).CreateIntent(context.Background(), CreateIntentInput{
		RiderID: store.ride.RiderID, RideID: store.ride.ID, Provider: "vpos", IdempotencyKey: "cash",
	})
	if err == nil {
		t.Fatal("cash ride accepted for online payment intent")
	}
}

type fakeCharger struct {
	result *IntentResult
	err    error
	calls  int
}

func (c *fakeCharger) CreatePaymentIntent(context.Context, uuid.UUID, int64, string) (*IntentResult, error) {
	c.calls++
	return c.result, c.err
}

func chargedStore() (*fakeStore, *fakeCharger) {
	riderID, rideID := uuid.New(), uuid.New()
	store := &fakeStore{ride: db.Ride{
		ID: rideID, RiderID: riderID, PaymentMethod: "card", Currency: "AOA",
		SuggestedFareCents: NumericFromInt64(1800),
	}}
	return store, &fakeCharger{result: &IntentResult{ProviderChargeID: "APPY-123", Status: "pending"}}
}

func TestCreateIntentWithChargerPersistsProviderChargeID(t *testing.T) {
	store, charger := chargedStore()
	service := NewService(store).WithCharger(charger)

	charge, err := service.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: store.ride.RiderID, RideID: store.ride.ID, Provider: "appypay", IdempotencyKey: "checkout-2",
	})
	if err != nil {
		t.Fatalf("create intent with charger: %v", err)
	}
	if charger.calls != 1 {
		t.Fatalf("charger calls = %d, want 1", charger.calls)
	}
	if store.createdIntent.ProviderChargeID != "APPY-123" {
		t.Fatalf("persisted provider charge id = %q, want APPY-123", store.createdIntent.ProviderChargeID)
	}
	if charge.ProviderChargeID != "APPY-123" {
		t.Fatalf("returned charge id = %q, want APPY-123", charge.ProviderChargeID)
	}
}

func TestCreateIntentChargerFailurePersistsNothing(t *testing.T) {
	store, charger := chargedStore()
	charger.err = context.DeadlineExceeded
	service := NewService(store).WithCharger(charger)

	_, err := service.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: store.ride.RiderID, RideID: store.ride.ID, Provider: "appypay", IdempotencyKey: "checkout-3",
	})
	if err == nil {
		t.Fatal("expected provider failure to fail intent creation")
	}
	if store.createdIntent != (db.CreatePaymentIntentParams{}) {
		t.Fatalf("intent persisted despite provider failure: %#v", store.createdIntent)
	}
}

func TestCreateIntentChargerRequiresChargeIdentifier(t *testing.T) {
	store, charger := chargedStore()
	charger.result = &IntentResult{}
	service := NewService(store).WithCharger(charger)

	_, err := service.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: store.ride.RiderID, RideID: store.ride.ID, Provider: "appypay", IdempotencyKey: "checkout-4",
	})
	if err == nil {
		t.Fatal("expected missing charge identifier to fail intent creation")
	}
	if store.createdIntent != (db.CreatePaymentIntentParams{}) {
		t.Fatalf("intent persisted without provider charge id: %#v", store.createdIntent)
	}
}

func TestCreateIntentWithoutChargerUsesOpaqueLocalID(t *testing.T) {
	store := &fakeStore{ride: db.Ride{
		ID: uuid.New(), RiderID: uuid.New(), PaymentMethod: "card", Currency: "AOA",
		SuggestedFareCents: NumericFromInt64(1800),
	}}
	service := NewService(store)

	charge, err := service.CreateIntent(context.Background(), CreateIntentInput{
		RiderID: store.ride.RiderID, RideID: store.ride.ID, Provider: "vpos", IdempotencyKey: "checkout-5",
	})
	if err != nil {
		t.Fatalf("ledger-only intent creation: %v", err)
	}
	if !strings.HasPrefix(charge.ProviderChargeID, "intent_") {
		t.Fatalf("ledger-only charge id = %q, want opaque intent_ prefix", charge.ProviderChargeID)
	}
}
