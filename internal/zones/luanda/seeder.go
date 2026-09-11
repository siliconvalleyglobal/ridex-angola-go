package luanda

import (
	"context"

	"github.com/ridex/ridex-angola/internal/db"
	"go.uber.org/zap"
)

// Seeder handles seeding Luanda zones into the database.
type Seeder struct {
	queries *db.Queries
	logger  *zap.Logger
}

// NewSeeder creates a new zone seeder.
func NewSeeder(queries *db.Queries, logger *zap.Logger) *Seeder {
	return &Seeder{
		queries: queries,
		logger:  logger,
	}
}

// Seed creates all Luanda zones if they don't already exist.
func (s *Seeder) Seed(ctx context.Context) error {
	for _, zone := range LuandaZones() {
		if err := s.seedZone(ctx, zone); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to seed zone",
					zap.String("zone", zone.Name),
					zap.Error(err),
				)
			}
		}
	}
	return nil
}

// seedZone creates a zone. If it already exists, it will fail with a unique constraint error which we ignore.
func (s *Seeder) seedZone(ctx context.Context, zone LuandaZone) error {
	params := zone.ToDBParams()
	_, err := s.queries.CreateServiceZone(ctx, params)
	if err != nil {
		// Zone already exists - this is fine
		if s.logger != nil {
			s.logger.Debug("zone already exists, skipping",
				zap.String("zone", zone.Name),
				zap.String("slug", zone.Slug),
			)
		}
		return nil
	}

	if s.logger != nil {
		s.logger.Info("seeded zone",
			zap.String("zone", zone.Name),
			zap.String("slug", zone.Slug),
			zap.Float64("minLat", zone.MinLat),
			zap.Float64("maxLat", zone.MaxLat),
			zap.Float64("minLng", zone.MinLng),
			zap.Float64("maxLng", zone.MaxLng),
			zap.Int64("baseCents", zone.BaseCents),
			zap.Int64("perKmCents", zone.PerKmCents),
		)
	}

	return nil
}
