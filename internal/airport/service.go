package airport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
	"go.uber.org/zap"
)

var (
	ErrAirportNotFound         = errors.New("airport not found")
	ErrTransferNotFound        = errors.New("airport transfer not found")
	ErrInvalidFlightNumber     = errors.New("invalid flight number")
	ErrTransferNotActive       = errors.New("transfer is not active")
	ErrAlreadyCompleted        = errors.New("transfer already completed")
	ErrInvalidStatusTransition = errors.New("invalid transfer status transition")
	ErrRideNotFound            = errors.New("ride not found")
	ErrForbidden               = errors.New("transfer is not accessible to the caller")
)

// Store is the database surface used by the airport transfer domain. Keeping
// it narrow makes the domain safe to exercise without a live database.
type Store interface {
	ListAirports(context.Context) ([]db.Airport, error)
	GetAirport(context.Context, uuid.UUID) (db.Airport, error)
	GetAirportByIATA(context.Context, string) (db.Airport, error)
	ListAirportTransfers(context.Context) ([]db.AirportTransfer, error)
	GetAirportTransfer(context.Context, uuid.UUID) (db.AirportTransfer, error)
	GetAirportTransferByRideID(context.Context, uuid.UUID) (db.AirportTransfer, error)
	CreateAirportTransfer(context.Context, db.CreateAirportTransferParams) (db.AirportTransfer, error)
	UpdateAirportTransfer(context.Context, db.UpdateAirportTransferParams) (db.AirportTransfer, error)
	CancelAirportTransfer(context.Context, db.CancelAirportTransferParams) (db.AirportTransfer, error)
	GetRideByID(context.Context, uuid.UUID) (db.Ride, error)
}

// Service handles airport and airport-transfer operations.
type Service struct {
	store          Store
	logger         *zap.Logger
	flightProvider FlightProvider // optional external flight-status lookup
}

// NewService creates a new airport service wired to the provided store.
func NewService(store Store, logger *zap.Logger) *Service {
	return &Service{store: store, logger: logger}
}

// WithFlightProvider attaches an optional flight-status provider so transfer
// creation can record observed flight information without blocking the ride.
func (s *Service) WithFlightProvider(provider FlightProvider) *Service {
	s.flightProvider = provider
	return s
}

// ValidateFlightNumber checks if a flight number is valid (IATA-like: two
// airline letters followed by 1–3 digits, e.g. DT123 or TA001).
func ValidateFlightNumber(flightNumber string) bool {
	if len(flightNumber) < 3 || len(flightNumber) > 5 {
		return false
	}
	if flightNumber[0] < 'A' || flightNumber[0] > 'Z' {
		return false
	}
	if flightNumber[1] < 'A' || flightNumber[1] > 'Z' {
		return false
	}
	for i := 2; i < len(flightNumber); i++ {
		if flightNumber[i] < '0' || flightNumber[i] > '9' {
			return false
		}
	}
	return true
}

// ListAirports returns all airports.
func (s *Service) ListAirports(ctx context.Context) ([]Airport, error) {
	rows, err := s.store.ListAirports(ctx)
	if err != nil {
		return nil, fmt.Errorf("list airports: %w", err)
	}
	out := make([]Airport, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromDBAirport(row))
	}
	return out, nil
}

// GetAirport returns an airport by ID.
func (s *Service) GetAirport(ctx context.Context, id uuid.UUID) (*Airport, error) {
	row, err := s.store.GetAirport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAirportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get airport: %w", err)
	}
	airport := fromDBAirport(row)
	return &airport, nil
}

// GetAirportByIATA returns an airport by IATA code.
func (s *Service) GetAirportByIATA(ctx context.Context, iataCode string) (*Airport, error) {
	row, err := s.store.GetAirportByIATA(ctx, iataCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAirportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get airport by IATA: %w", err)
	}
	airport := fromDBAirport(row)
	return &airport, nil
}

// CreateTransfer creates a new airport transfer for a ride the caller owns.
// When a flight number is supplied it is validated locally; if a
// FlightProvider is configured it is consulted for live status but a lookup
// failure never blocks the transfer.
func (s *Service) CreateTransfer(ctx context.Context, callerID uuid.UUID, input AirportTransferCreateInput) (*AirportTransfer, error) {
	if callerID == uuid.Nil {
		return nil, ErrForbidden
	}
	if input.FlightNumber != "" && !ValidateFlightNumber(input.FlightNumber) {
		return nil, ErrInvalidFlightNumber
	}
	ride, err := s.store.GetRideByID(ctx, input.RideID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRideNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load ride: %w", err)
	}
	if !CanAccessTransfer(callerID, ride) {
		return nil, ErrForbidden
	}
	if _, err := s.store.GetAirport(ctx, input.AirportID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAirportNotFound
		}
		return nil, fmt.Errorf("verify airport: %w", err)
	}
	if input.FlightNumber != "" && s.flightProvider != nil {
		if info, err := s.flightProvider.GetFlightStatus(ctx, input.FlightNumber); err == nil && info != nil {
			s.logger.Debug("flight status observed",
				zap.String("flight_number", input.FlightNumber),
				zap.String("status", string(info.Status)))
		} else {
			s.logger.Warn("flight status lookup failed",
				zap.String("flight_number", input.FlightNumber), zap.Error(err))
		}
	}
	row, err := s.store.CreateAirportTransfer(ctx, db.CreateAirportTransferParams{
		RideID:           input.RideID,
		AirportID:        input.AirportID,
		IsPickup:         input.IsPickup,
		FlightNumber:     pgtext(input.FlightNumber),
		Airline:          pgtext(input.Airline),
		ScheduledArrival: pgts(input.ScheduledArrival),
		ActualArrival:    pgtype.Timestamptz{},
		PassengerName:    pgtext(input.PassengerName),
		PassengerPhone:   pgtext(input.PassengerPhone),
		MeetAndGreet:     input.MeetAndGreet,
		WaitingMinutes:   int32(input.WaitingMinutes),
		WaitingFeeCents:  int32(input.WaitingFeeCents),
		FixedFareCents:   int32(input.FixedFareCents),
		Notes:            pgtext(input.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("create airport transfer: %w", err)
	}
	transfer := fromDBTransfer(row)
	return &transfer, nil
}

// ListTransfers returns all airport transfers.
func (s *Service) ListTransfers(ctx context.Context) ([]AirportTransfer, error) {
	rows, err := s.store.ListAirportTransfers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list airport transfers: %w", err)
	}
	out := make([]AirportTransfer, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromDBTransfer(row))
	}
	return out, nil
}

// GetTransfer returns an airport transfer by ID.
func (s *Service) GetTransfer(ctx context.Context, id uuid.UUID) (*AirportTransfer, error) {
	row, err := s.store.GetAirportTransfer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get airport transfer: %w", err)
	}
	transfer := fromDBTransfer(row)
	return &transfer, nil
}

// GetTransferByRideID returns the airport transfer linked to a ride, if any.
func (s *Service) GetTransferByRideID(ctx context.Context, rideID uuid.UUID) (*AirportTransfer, error) {
	row, err := s.store.GetAirportTransferByRideID(ctx, rideID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get airport transfer by ride: %w", err)
	}
	transfer := fromDBTransfer(row)
	return &transfer, nil
}

// UpdateTransfer applies a partial update, fetching the current row so a
// caller can change only the fields it cares about. Only the ride's rider or
// its assigned driver may mutate a transfer.
func (s *Service) UpdateTransfer(ctx context.Context, callerID uuid.UUID, transferID uuid.UUID, in AirportTransferUpdateInput) (*AirportTransfer, error) {
	current, err := s.store.GetAirportTransfer(ctx, transferID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load airport transfer: %w", err)
	}
	if err := s.authorizeParticipant(ctx, callerID, current.RideID); err != nil {
		return nil, err
	}

	status := current.Status
	if in.Status != nil {
		status = db.AirportTransferStatus(*in.Status)
	}
	if err := validateTransition(current.Status, status); err != nil {
		return nil, err
	}
	actualArrival := current.ActualArrival
	if in.ActualArrival != nil {
		actualArrival = pgts(in.ActualArrival)
	}
	waitingMinutes := current.WaitingMinutes
	if in.WaitingMinutes != nil {
		waitingMinutes = int32(*in.WaitingMinutes)
	}
	notes := current.Notes
	if in.Notes != nil {
		notes = pgtext(*in.Notes)
	}

	row, err := s.store.UpdateAirportTransfer(ctx, db.UpdateAirportTransferParams{
		ID:             transferID,
		Status:         status,
		ActualArrival:  actualArrival,
		WaitingMinutes: waitingMinutes,
		Notes:          notes,
	})
	if err != nil {
		return nil, fmt.Errorf("update airport transfer: %w", err)
	}
	transfer := fromDBTransfer(row)
	return &transfer, nil
}

// UpdateTransferStatus updates the status of an airport transfer.
func (s *Service) UpdateTransferStatus(ctx context.Context, callerID uuid.UUID, transferID uuid.UUID, status AirportTransferStatus) error {
	_, err := s.UpdateTransfer(ctx, callerID, transferID, AirportTransferUpdateInput{Status: &status})
	return err
}

// CalculateWaitingFee calculates the waiting fee for an airport transfer.
func CalculateWaitingFee(waitingMinutes int, perMinuteRate int64) int64 {
	if waitingMinutes < 0 {
		waitingMinutes = 0
	}
	if perMinuteRate < 0 {
		perMinuteRate = 0
	}
	return int64(waitingMinutes) * perMinuteRate
}

// CalculateFixedFare calculates the fixed fare for an airport transfer.
func CalculateFixedFare(baseFare int64, meetAndGreet bool, meetAndGreetFee int64) int64 {
	total := baseFare
	if meetAndGreet {
		total += meetAndGreetFee
	}
	return total
}

// validateTransition enforces the accepted airport-transfer lifecycle:
// terminal states (completed, cancelled, no_show) are final and reject any
// further transition.
func validateTransition(current, next db.AirportTransferStatus) error {
	switch current {
	case db.AirportTransferStatusCompleted, db.AirportTransferStatusCancelled, db.AirportTransferStatusNoShow:
		return ErrInvalidStatusTransition
	}
	switch next {
	case db.AirportTransferStatusPending, db.AirportTransferStatusAssigned,
		db.AirportTransferStatusEnRoute, db.AirportTransferStatusArrived,
		db.AirportTransferStatusCompleted, db.AirportTransferStatusCancelled,
		db.AirportTransferStatusNoShow:
		return nil
	default:
		return ErrInvalidStatusTransition
	}
}

func pgtext(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func pgts(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func fromDBAirport(row db.Airport) Airport {
	return Airport{
		ID:            row.ID,
		IATA:          row.Iata,
		Name:          row.Name,
		City:          row.City,
		Country:       row.Country,
		CountryCode:   row.CountryCode,
		Latitude:      row.Latitude,
		Longitude:     row.Longitude,
		Timezone:      row.Timezone,
		TerminalCount: int(row.TerminalCount),
		PickupZones:   row.PickupZones,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

func fromDBTransfer(row db.AirportTransfer) AirportTransfer {
	return AirportTransfer{
		ID:               row.ID,
		RideID:           row.RideID,
		AirportID:        row.AirportID,
		IsPickup:         row.IsPickup,
		FlightNumber:     row.FlightNumber.String,
		Airline:          row.Airline.String,
		ScheduledArrival: tsPtr(row.ScheduledArrival),
		ActualArrival:    tsPtr(row.ActualArrival),
		PassengerName:    row.PassengerName.String,
		PassengerPhone:   row.PassengerPhone.String,
		MeetAndGreet:     row.MeetAndGreet,
		WaitingMinutes:   int(row.WaitingMinutes),
		WaitingFeeCents:  int64(row.WaitingFeeCents),
		FixedFareCents:   int64(row.FixedFareCents),
		Status:           AirportTransferStatus(row.Status),
		Notes:            row.Notes.String,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func tsPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

// RecordActualArrival records the actual arrival time of a flight.
func (s *Service) RecordActualArrival(ctx context.Context, callerID uuid.UUID, transferID uuid.UUID, actualArrival time.Time) error {
	_, err := s.UpdateTransfer(ctx, callerID, transferID, AirportTransferUpdateInput{ActualArrival: &actualArrival})
	return err
}

// CancelTransfer cancels an active airport transfer. Transfers that already
// reached a terminal state cannot be cancelled.
func (s *Service) CancelTransfer(ctx context.Context, callerID uuid.UUID, transferID uuid.UUID, reason string) (*AirportTransfer, error) {
	current, err := s.store.GetAirportTransfer(ctx, transferID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load airport transfer: %w", err)
	}
	if err := s.authorizeParticipant(ctx, callerID, current.RideID); err != nil {
		return nil, err
	}
	row, err := s.store.CancelAirportTransfer(ctx, db.CancelAirportTransferParams{
		ID: transferID, Reason: reason,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAlreadyCompleted
	}
	if err != nil {
		return nil, fmt.Errorf("cancel airport transfer: %w", err)
	}
	transfer := fromDBTransfer(row)
	return &transfer, nil
}

// authorizeParticipant verifies the caller is the ride's rider or its
// assigned driver before a transfer mutation is applied.
func (s *Service) authorizeParticipant(ctx context.Context, callerID uuid.UUID, rideID uuid.UUID) error {
	if callerID == uuid.Nil {
		return ErrForbidden
	}
	ride, err := s.store.GetRideByID(ctx, rideID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRideNotFound
	}
	if err != nil {
		return fmt.Errorf("load ride: %w", err)
	}
	if !CanAccessTransfer(callerID, ride) {
		return ErrForbidden
	}
	return nil
}

// CanAccessTransfer is kept pure so authorization remains easy to test and
// mirrors the participant rules used by the support and safety domains: the
// ride's rider and its assigned driver may manage its airport transfer.
func CanAccessTransfer(callerID uuid.UUID, ride db.Ride) bool {
	if ride.RiderID == callerID {
		return true
	}
	if ride.DriverID.Valid && uuid.UUID(ride.DriverID.Bytes) == callerID {
		return true
	}
	return false
}
