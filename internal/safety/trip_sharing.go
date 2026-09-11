package safety

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// TripShare represents a live trip sharing session.
type TripShare struct {
	ID         uuid.UUID
	RideID     uuid.UUID
	RiderID    uuid.UUID
	SharedWith []uuid.UUID
	ShareURL   string
	Active     bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// LocationUpdate represents a real-time location update.
type LocationUpdate struct {
	RideID    uuid.UUID
	Latitude  float64
	Longitude float64
	SpeedKmh  float64
	Heading   float64
	Timestamp time.Time
}

// TripSnapshot represents a point-in-time trip status.
type TripSnapshot struct {
	RideID            uuid.UUID
	Status            string
	DriverLocation    *LocationUpdate
	ETA               time.Duration
	DistanceRemaining float64
	UpdatedAt         time.Time
}

// TripSharingService manages trip sharing operations.
type TripSharingService struct {
	q *db.Queries
}

// NewTripSharingService creates a new trip sharing service.
func NewTripSharingService(q *db.Queries) *TripSharingService {
	return &TripSharingService{q: q}
}

// StartSharing begins a trip sharing session.
func (s *TripSharingService) StartSharing(ctx context.Context, rideID, riderID uuid.UUID, contacts []uuid.UUID) (*TripShare, error) {
	share := &TripShare{
		ID:         uuid.New(),
		RideID:     rideID,
		RiderID:    riderID,
		SharedWith: contacts,
		Active:     true,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(2 * time.Hour),
	}

	// Persist to database
	_, err := s.q.CreateTripShare(ctx, db.CreateTripShareParams{
		RideID:    rideID,
		RiderID:   riderID,
		TokenHash: []byte(share.ID.String()),
		ExpiresAt: pgtype.Timestamptz{Time: share.ExpiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return share, nil
}

// StopSharing ends a trip sharing session.
func (s *TripSharingService) StopSharing(ctx context.Context, rideID uuid.UUID) error {
	return s.q.DeactivateTripShare(ctx, rideID)
}

// GetSharedLocations returns locations for shared trip viewers.
func (s *TripSharingService) GetSharedLocations(ctx context.Context, rideID uuid.UUID) (*TripSnapshot, error) {
	ride, err := s.q.GetRideByID(ctx, rideID)
	if err != nil {
		return nil, err
	}

	driverID, _ := uuid.FromBytes(ride.DriverID.Bytes[:])
	location, err := s.q.GetDriverLocation(ctx, driverID)
	if err != nil {
		return nil, err
	}

	return &TripSnapshot{
		RideID: rideID,
		Status: ride.Status,
		DriverLocation: &LocationUpdate{
			Latitude:  location.Location.P.Y,
			Longitude: location.Location.P.X,
			Timestamp: location.UpdatedAt.Time,
		},
		UpdatedAt: time.Now(),
	}, nil
}

// IsRouteDeviation checks if driver has deviated from expected route.
func IsRouteDeviation(plannedDistance, actualDistance float64) bool {
	if plannedDistance == 0 {
		return false
	}
	deviation := (actualDistance - plannedDistance) / plannedDistance
	return deviation > 0.3 // 30% deviation threshold
}
