package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// SavedPlace represents a user's saved location.
type SavedPlace struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Address   string
	Latitude  float64
	Longitude float64
	Icon      string
	CreatedAt pgtype.Timestamptz
}

// PlaceIcon represents common saved place icons.
const (
	PlaceIconHome     = "home"
	PlaceIconWork     = "work"
	PlaceIconGym      = "gym"
	PlaceIconSchool   = "school"
	PlaceIconAirport  = "airport"
	PlaceIconHospital = "hospital"
	PlaceIconOther    = "other"
)

// PlaceService handles saved place operations.
type PlaceService struct {
	q *db.Queries
}

// NewPlaceService creates a new place service.
func NewPlaceService(q *db.Queries) *PlaceService {
	return &PlaceService{q: q}
}

// CreatePlace adds a new saved place for a user.
func (s *PlaceService) CreatePlace(ctx context.Context, userID uuid.UUID, name, address string, lat, lng float64, icon string) (*SavedPlace, error) {
	place, err := s.q.CreateSavedPlace(ctx, db.CreateSavedPlaceParams{
		UserID:    userID,
		Name:      name,
		Address:   address,
		Latitude:  lat,
		Longitude: lng,
		Icon:      icon,
	})
	if err != nil {
		return nil, err
	}
	return &SavedPlace{
		ID:        place.ID,
		UserID:    place.UserID,
		Name:      place.Name,
		Address:   place.Address,
		Latitude:  place.Latitude,
		Longitude: place.Longitude,
		Icon:      place.Icon,
		CreatedAt: place.CreatedAt,
	}, nil
}

// GetPlaces returns all saved places for a user.
func (s *PlaceService) GetPlaces(ctx context.Context, userID uuid.UUID) ([]SavedPlace, error) {
	places, err := s.q.ListSavedPlaces(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]SavedPlace, 0, len(places))
	for _, p := range places {
		result = append(result, SavedPlace{
			ID:        p.ID,
			UserID:    p.UserID,
			Name:      p.Name,
			Address:   p.Address,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Icon:      p.Icon,
			CreatedAt: p.CreatedAt,
		})
	}
	return result, nil
}

// UpdatePlace updates a saved place.
func (s *PlaceService) UpdatePlace(ctx context.Context, placeID, userID uuid.UUID, name, address string, lat, lng float64, icon string) (*SavedPlace, error) {
	place, err := s.q.UpdateSavedPlace(ctx, db.UpdateSavedPlaceParams{
		ID:        placeID,
		UserID:    userID,
		Name:      name,
		Address:   address,
		Latitude:  lat,
		Longitude: lng,
		Icon:      icon,
	})
	if err != nil {
		return nil, err
	}
	return &SavedPlace{
		ID:        place.ID,
		UserID:    place.UserID,
		Name:      place.Name,
		Address:   place.Address,
		Latitude:  place.Latitude,
		Longitude: place.Longitude,
		Icon:      place.Icon,
		CreatedAt: place.CreatedAt,
	}, nil
}

// DeletePlace removes a saved place.
func (s *PlaceService) DeletePlace(ctx context.Context, placeID, userID uuid.UUID) error {
	return s.q.DeleteSavedPlace(ctx, db.DeleteSavedPlaceParams{
		ID:     placeID,
		UserID: userID,
	})
}

// QuickBooking represents a one-tap booking request from a saved place.
type QuickBooking struct {
	PlaceID     uuid.UUID
	UserID      uuid.UUID
	Pickup      string
	Destination string
}

// SuggestedPlace represents a place suggestion based on context.
type SuggestedPlace struct {
	Place  SavedPlace
	Reason string // "frequent", "time_based", "location_based"
	Score  float64
}
