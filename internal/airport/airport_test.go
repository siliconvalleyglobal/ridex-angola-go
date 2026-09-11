package airport

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
	"go.uber.org/zap"
)

func setupTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Sync() })
	return logger
}

func dbAirport(iata string) db.Airport {
	return db.Airport{
		ID: uuid.New(), Iata: iata, Name: "4 de Fevereiro International Airport",
		City: "Luanda", Country: "Angola", CountryCode: "AO",
		Latitude: -8.8426, Longitude: 13.2346, Timezone: "Africa/Luanda",
		TerminalCount: 1, PickupZones: []string{"arrivals_hall"},
	}
}

func dbRide(riderID uuid.UUID) db.Ride {
	return db.Ride{
		ID: uuid.New(), RiderID: riderID, Status: "requested",
		Currency: "AOA", PaymentMethod: "card",
	}
}

func dbTransfer(id uuid.UUID, status db.AirportTransferStatus) db.AirportTransfer {
	return db.AirportTransfer{
		ID: id, RideID: uuid.New(), AirportID: uuid.New(),
		FlightNumber: pgtype.Text{String: "DT123", Valid: true},
		Status:       status,
	}
}

// fakeStore implements airport.Store for tests.
type fakeStore struct {
	mu        sync.Mutex
	airports  []db.Airport
	transfers []db.AirportTransfer
	rides     []db.Ride
	err       error // optional global error
}

func newFakeStore() *fakeStore {
	riderID := uuid.New()
	ride := dbRide(riderID)
	transfer := dbTransfer(uuid.New(), db.AirportTransferStatusPending)
	transfer.RideID = ride.ID
	return &fakeStore{
		airports:  []db.Airport{dbAirport("LAD")},
		transfers: []db.AirportTransfer{transfer},
		rides:     []db.Ride{ride},
	}
}

// riderID returns the rider owning the seeded ride.
func (f *fakeStore) riderID() uuid.UUID { return f.rides[0].RiderID }

func (f *fakeStore) ListAirports(context.Context) ([]db.Airport, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]db.Airport(nil), f.airports...), nil
}

func (f *fakeStore) GetAirport(_ context.Context, id uuid.UUID) (db.Airport, error) {
	if f.err != nil {
		return db.Airport{}, f.err
	}
	for _, a := range f.airports {
		if a.ID == id {
			return a, nil
		}
	}
	return db.Airport{}, pgx.ErrNoRows
}

func (f *fakeStore) GetAirportByIATA(_ context.Context, iata string) (db.Airport, error) {
	if f.err != nil {
		return db.Airport{}, f.err
	}
	for _, a := range f.airports {
		if a.Iata == iata {
			return a, nil
		}
	}
	return db.Airport{}, pgx.ErrNoRows
}

func (f *fakeStore) ListAirportTransfers(context.Context) ([]db.AirportTransfer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]db.AirportTransfer(nil), f.transfers...), nil
}

func (f *fakeStore) GetAirportTransfer(_ context.Context, id uuid.UUID) (db.AirportTransfer, error) {
	if f.err != nil {
		return db.AirportTransfer{}, f.err
	}
	for _, t := range f.transfers {
		if t.ID == id {
			return t, nil
		}
	}
	return db.AirportTransfer{}, pgx.ErrNoRows
}

func (f *fakeStore) GetAirportTransferByRideID(_ context.Context, rideID uuid.UUID) (db.AirportTransfer, error) {
	for _, t := range f.transfers {
		if t.RideID == rideID {
			return t, nil
		}
	}
	return db.AirportTransfer{}, pgx.ErrNoRows
}

func (f *fakeStore) CreateAirportTransfer(_ context.Context, arg db.CreateAirportTransferParams) (db.AirportTransfer, error) {
	row := dbTransfer(uuid.New(), db.AirportTransferStatusPending)
	row.RideID = arg.RideID
	row.AirportID = arg.AirportID
	row.IsPickup = arg.IsPickup
	row.FlightNumber = arg.FlightNumber
	row.Airline = arg.Airline
	row.ScheduledArrival = arg.ScheduledArrival
	row.PassengerName = arg.PassengerName
	row.PassengerPhone = arg.PassengerPhone
	row.MeetAndGreet = arg.MeetAndGreet
	row.WaitingMinutes = arg.WaitingMinutes
	row.WaitingFeeCents = arg.WaitingFeeCents
	row.FixedFareCents = arg.FixedFareCents
	row.Notes = arg.Notes
	f.mu.Lock()
	f.transfers = append(f.transfers, row)
	f.mu.Unlock()
	return row, nil
}

func (f *fakeStore) UpdateAirportTransfer(_ context.Context, arg db.UpdateAirportTransferParams) (db.AirportTransfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.transfers {
		if f.transfers[i].ID == arg.ID {
			f.transfers[i].Status = arg.Status
			f.transfers[i].ActualArrival = arg.ActualArrival
			f.transfers[i].WaitingMinutes = arg.WaitingMinutes
			f.transfers[i].Notes = arg.Notes
			return f.transfers[i], nil
		}
	}
	return db.AirportTransfer{}, pgx.ErrNoRows
}

func (f *fakeStore) CancelAirportTransfer(_ context.Context, arg db.CancelAirportTransferParams) (db.AirportTransfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.transfers {
		if f.transfers[i].ID == arg.ID {
			switch f.transfers[i].Status {
			case db.AirportTransferStatusCompleted, db.AirportTransferStatusCancelled, db.AirportTransferStatusNoShow:
				return db.AirportTransfer{}, pgx.ErrNoRows
			}
			f.transfers[i].Status = db.AirportTransferStatusCancelled
			if arg.Reason != "" {
				f.transfers[i].Notes = pgtype.Text{String: arg.Reason, Valid: true}
			}
			return f.transfers[i], nil
		}
	}
	return db.AirportTransfer{}, pgx.ErrNoRows
}

func (f *fakeStore) GetRideByID(_ context.Context, id uuid.UUID) (db.Ride, error) {
	if f.err != nil {
		return db.Ride{}, f.err
	}
	for _, r := range f.rides {
		if r.ID == id {
			return r, nil
		}
	}
	return db.Ride{}, pgx.ErrNoRows
}

func TestAirportService_ListAirports(t *testing.T) {
	logger := setupTestLogger(t)
	svc := NewService(newFakeStore(), logger)

	airports, err := svc.ListAirports(context.Background())
	if err != nil {
		t.Fatalf("ListAirports failed: %v", err)
	}
	if len(airports) != 1 {
		t.Fatalf("expected 1 airport, got %d", len(airports))
	}
	if airports[0].IATA != "LAD" {
		t.Errorf("expected IATA LAD, got %s", airports[0].IATA)
	}
}

func TestAirportService_GetAirportByIATA(t *testing.T) {
	logger := setupTestLogger(t)
	svc := NewService(newFakeStore(), logger)

	airport, err := svc.GetAirportByIATA(context.Background(), "LAD")
	if err != nil {
		t.Fatalf("GetAirportByIATA failed: %v", err)
	}
	if airport.IATA != "LAD" {
		t.Errorf("expected LAD, got %s", airport.IATA)
	}
}

func TestAirportService_GetAirportByIATA_NotFound(t *testing.T) {
	logger := setupTestLogger(t)
	svc := NewService(newFakeStore(), logger)

	if _, err := svc.GetAirportByIATA(context.Background(), "XXX"); !errors.Is(err, ErrAirportNotFound) {
		t.Fatalf("expected ErrAirportNotFound, got %v", err)
	}
}

func TestAirportService_CreateTransfer(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	scheduled := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	transfer, err := svc.CreateTransfer(context.Background(), store.riderID(), AirportTransferCreateInput{
		RideID:           store.rides[0].ID,
		AirportID:        store.airports[0].ID,
		IsPickup:         true,
		FlightNumber:     "DT123",
		Airline:          "TAAG",
		ScheduledArrival: &scheduled,
		PassengerName:    "Joao Silva",
		PassengerPhone:   "+244912345678",
		MeetAndGreet:     false,
		FixedFareCents:   8000,
		Notes:            "Nenhuma",
	})
	if err != nil {
		t.Fatalf("CreateTransfer failed: %v", err)
	}
	if transfer == nil {
		t.Fatal("expected non-nil transfer")
	}
	if transfer.ID == uuid.Nil {
		t.Error("expected non-empty transfer ID")
	}
	if transfer.FlightNumber != "DT123" {
		t.Errorf("expected flight number DT123, got %s", transfer.FlightNumber)
	}
	if transfer.Status != AirportTransferStatusPending {
		t.Errorf("expected status pending, got %s", transfer.Status)
	}
	if transfer.WaitingFeeCents != 0 || transfer.FixedFareCents != 8000 {
		t.Errorf("unexpected fees: waiting=%d fixed=%d", transfer.WaitingFeeCents, transfer.FixedFareCents)
	}
}

func TestAirportService_CreateTransfer_UnknownAirport(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	_, err := svc.CreateTransfer(context.Background(), store.riderID(), AirportTransferCreateInput{
		RideID: store.rides[0].ID, AirportID: uuid.New(),
	})
	if !errors.Is(err, ErrAirportNotFound) {
		t.Fatalf("expected ErrAirportNotFound, got %v", err)
	}
}

func TestAirportService_CreateTransfer_InvalidFlight(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	_, err := svc.CreateTransfer(context.Background(), store.riderID(), AirportTransferCreateInput{
		RideID: store.rides[0].ID, AirportID: store.airports[0].ID,
		FlightNumber: "bad",
	})
	if !errors.Is(err, ErrInvalidFlightNumber) {
		t.Fatalf("expected ErrInvalidFlightNumber, got %v", err)
	}
}

func TestAirportService_CreateTransfer_ForbiddenRider(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	_, err := svc.CreateTransfer(context.Background(), uuid.New(), AirportTransferCreateInput{
		RideID: store.rides[0].ID, AirportID: store.airports[0].ID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-participant, got %v", err)
	}
}

func TestAirportService_CreateTransfer_RideNotFound(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	_, err := svc.CreateTransfer(context.Background(), store.riderID(), AirportTransferCreateInput{
		RideID: uuid.New(), AirportID: store.airports[0].ID,
	})
	if !errors.Is(err, ErrRideNotFound) {
		t.Fatalf("expected ErrRideNotFound, got %v", err)
	}
}

func TestAirportService_FareHelpers(t *testing.T) {
	if got := CalculateWaitingFee(45, 100); got != 4500 {
		t.Errorf("expected waiting fee 4500, got %d", got)
	}
	if got := CalculateWaitingFee(-5, 100); got != 0 {
		t.Errorf("expected negative waiting clamped to 0, got %d", got)
	}
	if got := CalculateFixedFare(8000, true, 2000); got != 10000 {
		t.Errorf("expected fixed fare 10000, got %d", got)
	}
	if got := CalculateFixedFare(8000, false, 2000); got != 8000 {
		t.Errorf("expected fixed fare 8000, got %d", got)
	}
}

func TestValidateFlightNumber(t *testing.T) {
	tests := []struct {
		flightNumber string
		wantValid    bool
	}{
		{"DT123", true},
		{"TA001", true},
		{"D8123", false},
		{"001TA", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.flightNumber, func(t *testing.T) {
			if valid := ValidateFlightNumber(tt.flightNumber); valid != tt.wantValid {
				t.Errorf("expected valid=%v, got %v for flight %q", tt.wantValid, valid, tt.flightNumber)
			}
		})
	}
}

func TestAirportService_UpdateStatusAndArrival(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)
	ctx := context.Background()
	rider := store.riderID()

	id := store.transfers[0].ID
	if err := svc.UpdateTransferStatus(ctx, rider, id, AirportTransferStatusEnRoute); err != nil {
		t.Fatalf("UpdateTransferStatus failed: %v", err)
	}
	arrival := time.Now().UTC()
	if err := svc.RecordActualArrival(ctx, rider, id, arrival); err != nil {
		t.Fatalf("RecordActualArrival failed: %v", err)
	}
	transfer, err := svc.GetTransfer(ctx, id)
	if err != nil {
		t.Fatalf("GetTransfer failed: %v", err)
	}
	if transfer.Status != AirportTransferStatusEnRoute {
		t.Errorf("expected en_route status, got %s", transfer.Status)
	}
	if transfer.ActualArrival == nil || !transfer.ActualArrival.Equal(arrival) {
		t.Errorf("expected actual arrival %v, got %v", arrival, transfer.ActualArrival)
	}
}

func TestAirportService_TerminalStatusRejectsFurtherTransition(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)
	ctx := context.Background()
	rider := store.riderID()

	id := store.transfers[0].ID
	if err := svc.UpdateTransferStatus(ctx, rider, id, AirportTransferStatusCompleted); err != nil {
		t.Fatalf("mark completed failed: %v", err)
	}
	if err := svc.UpdateTransferStatus(ctx, rider, id, AirportTransferStatusEnRoute); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("expected ErrInvalidStatusTransition after completion, got %v", err)
	}
}

func TestAirportService_CancelTransfer(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)
	ctx := context.Background()
	rider := store.riderID()

	id := store.transfers[0].ID
	transfer, err := svc.CancelTransfer(ctx, rider, id, "order canceled")
	if err != nil {
		t.Fatalf("CancelTransfer failed: %v", err)
	}
	if transfer.Status != AirportTransferStatusCancelled {
		t.Errorf("expected cancelled status, got %s", transfer.Status)
	}
	if transfer.Notes != "order canceled" {
		t.Errorf("expected cancel reason notes, got %q", transfer.Notes)
	}
}

func TestCanAccessTransfer(t *testing.T) {
	rider := uuid.New()
	driver := uuid.New()
	ride := db.Ride{RiderID: rider, DriverID: pgtype.UUID{Bytes: driver, Valid: true}}

	if !CanAccessTransfer(rider, ride) {
		t.Error("ride rider should have access")
	}
	if !CanAccessTransfer(driver, ride) {
		t.Error("assigned driver should have access")
	}
	if CanAccessTransfer(uuid.New(), ride) {
		t.Error("unrelated user should not have access")
	}
	rideWithoutDriver := db.Ride{RiderID: rider}
	if CanAccessTransfer(driver, rideWithoutDriver) {
		t.Error("driver should not have access when unassigned")
	}
}

func TestAirportService_AssignedDriverMayUpdate(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)
	ctx := context.Background()

	driver := uuid.New()
	store.mu.Lock()
	store.rides[0].DriverID = pgtype.UUID{Bytes: driver, Valid: true}
	store.mu.Unlock()

	id := store.transfers[0].ID
	if err := svc.UpdateTransferStatus(ctx, driver, id, AirportTransferStatusArrived); err != nil {
		t.Fatalf("assigned driver update failed: %v", err)
	}
	if err := svc.UpdateTransferStatus(ctx, uuid.New(), id, AirportTransferStatusCompleted); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unrelated caller, got %v", err)
	}
}

func TestAirportService_CancelTransfer_ForbiddenCaller(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	id := store.transfers[0].ID
	if _, err := svc.CancelTransfer(context.Background(), uuid.New(), id, "nope"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unrelated caller, got %v", err)
	}
}

func TestAirportService_GetTransferByRideID(t *testing.T) {
	logger := setupTestLogger(t)
	store := newFakeStore()
	svc := NewService(store, logger)

	rideID := store.transfers[0].RideID
	transfer, err := svc.GetTransferByRideID(context.Background(), rideID)
	if err != nil {
		t.Fatalf("GetTransferByRideID failed: %v", err)
	}
	if transfer.RideID != rideID {
		t.Errorf("expected ride %s, got %s", rideID, transfer.RideID)
	}
	if _, err := svc.GetTransferByRideID(context.Background(), uuid.New()); !errors.Is(err, ErrTransferNotFound) {
		t.Fatalf("expected ErrTransferNotFound, got %v", err)
	}
}
