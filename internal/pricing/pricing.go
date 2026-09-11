package pricing

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// Service computes suggested fares from DB-driven config values.
type Service struct {
	q   *db.Queries
	ttl time.Duration

	mu       sync.RWMutex
	cached   rates
	cachedAt time.Time
}

// rates holds the fare parameters loaded from the config table.
type rates struct {
	BaseCents   int64
	PerKMCents  int64
	PerMinCents int64
}

// NewService creates a pricing service with a 1-minute config cache.
func NewService(q *db.Queries) *Service {
	return &Service{q: q, ttl: time.Minute}
}

// Quote is a fare estimate in cents with a breakdown.
type Quote struct {
	TotalCents    int64      `json:"totalCents"`
	Currency      string     `json:"currency"`
	DistanceKm    float64    `json:"distanceKm"`
	EstMinutes    float64    `json:"estMinutes"`
	BaseCents     int64      `json:"baseCents"`
	DistanceCents int64      `json:"distanceCents"`
	TimeCents     int64      `json:"timeCents"`
	ZoneID        *uuid.UUID `json:"zoneId,omitempty"`
}

// Suggest returns the fare for a trip between two coordinates.
// Distance uses the haversine formula; duration assumes a 25 km/h
// average city speed for Luanda traffic.
func (s *Service) Suggest(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (*Quote, error) {
	r, err := s.rates(ctx)
	if err != nil {
		return nil, err
	}
	var zoneID *uuid.UUID
	if zone, zoneErr := s.q.FindServiceZoneForPoint(ctx, db.FindServiceZoneForPointParams{
		Column1: decimalNumeric(fromLat), Column2: decimalNumeric(fromLng),
	}); zoneErr == nil {
		zoneID = &zone.ID
		if zone.BaseCents.Valid {
			r.BaseCents = numericCents(zone.BaseCents, r.BaseCents)
		}
		if zone.PerKmCents.Valid {
			r.PerKMCents = numericCents(zone.PerKmCents, r.PerKMCents)
		}
		if zone.PerMinCents.Valid {
			r.PerMinCents = numericCents(zone.PerMinCents, r.PerMinCents)
		}
	} else if !errors.Is(zoneErr, pgx.ErrNoRows) {
		return nil, zoneErr
	}

	distKm := haversineKm(fromLat, fromLng, toLat, toLng)
	minutes := distKm / 25.0 * 60.0 // 25 km/h average

	base := r.BaseCents
	dist := int64(math.Round(distKm * float64(r.PerKMCents)))
	tmin := int64(math.Round(minutes * float64(r.PerMinCents)))

	return &Quote{
		TotalCents:    base + dist + tmin,
		Currency:      "AOA",
		DistanceKm:    math.Round(distKm*100) / 100,
		EstMinutes:    math.Round(minutes*10) / 10,
		BaseCents:     base,
		DistanceCents: dist,
		TimeCents:     tmin,
		ZoneID:        zoneID,
	}, nil
}

// rates loads fare parameters from the config table with a short cache.
func (s *Service) rates(ctx context.Context) (rates, error) {
	s.mu.RLock()
	if time.Since(s.cachedAt) < s.ttl {
		r := s.cached
		s.mu.RUnlock()
		return r, nil
	}
	s.mu.RUnlock()

	rows, err := s.q.GetAllConfig(ctx)
	if err != nil {
		return rates{}, err
	}

	r := rates{BaseCents: 1000, PerKMCents: 500, PerMinCents: 30} // sane defaults
	for _, row := range rows {
		switch row.Key {
		case "fare_base_cents":
			r.BaseCents = atoi(row.Value, r.BaseCents)
		case "fare_per_km_cents":
			r.PerKMCents = atoi(row.Value, r.PerKMCents)
		case "fare_per_min_cents":
			r.PerMinCents = atoi(row.Value, r.PerMinCents)
		}
	}

	s.mu.Lock()
	s.cached, s.cachedAt = r, time.Now()
	s.mu.Unlock()
	return r, nil
}

// haversineKm returns great-circle distance in kilometres.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKm = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthKm * math.Asin(math.Sqrt(a))
}

func atoi(s string, fallback int64) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func decimalNumeric(v float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(math.Round(v * 1_000_000))), Exp: -6, Valid: true}
}

func numericCents(n pgtype.Numeric, fallback int64) int64 {
	if !n.Valid || n.Int == nil {
		return fallback
	}
	v := new(big.Float).SetInt(n.Int)
	f, _ := v.Float64()
	return int64(math.Round(f * math.Pow10(int(n.Exp))))
}
