package pricing

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SurgeMultiplier represents pricing multiplier for a zone.
type SurgeMultiplier struct {
	ZoneID      uuid.UUID
	Multiplier  float64
	DemandLevel string // low, normal, high, extreme
	ValidUntil  time.Time
	Reason      string
}

// DemandHeatmap represents demand data for a zone.
type DemandHeatmap struct {
	ZoneID           uuid.UUID
	ZoneName         string
	ActiveRiders     int
	AvailableDrivers int
	Ratio            float64
	SurgeMultiplier  float64
	DemandTrend      string // increasing, stable, decreasing
	UpdatedAt        time.Time
}

// SurgeConfig defines surge pricing parameters.
type SurgeConfig struct {
	Enabled           bool
	MinMultiplier     float64
	MaxMultiplier     float64
	ThresholdLow      float64
	ThresholdHigh     float64
	ThresholdExtreme  float64
	UpdateIntervalSec int
	DecayRate         float64
}

// DefaultSurgeConfig returns default surge configuration.
func DefaultSurgeConfig() SurgeConfig {
	return SurgeConfig{
		Enabled:           true,
		MinMultiplier:     1.0,
		MaxMultiplier:     3.0,
		ThresholdLow:      0.5,
		ThresholdHigh:     1.5,
		ThresholdExtreme:  2.5,
		UpdateIntervalSec: 60,
		DecayRate:         0.9,
	}
}

// SurgeService manages dynamic pricing.
type SurgeService struct {
	mu          sync.RWMutex
	config      SurgeConfig
	multipliers map[uuid.UUID]*SurgeMultiplier
}

// NewSurgeService creates a new surge pricing service.
func NewSurgeService(config SurgeConfig) *SurgeService {
	return &SurgeService{
		config:      config,
		multipliers: make(map[uuid.UUID]*SurgeMultiplier),
	}
}

// UpdateDemand updates demand data for a zone.
func (s *SurgeService) UpdateDemand(zoneID uuid.UUID, activeRiders, availableDrivers int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ratio := 0.0
	if availableDrivers > 0 {
		ratio = float64(activeRiders) / float64(availableDrivers)
	}

	multiplier := calculateMultiplier(ratio, s.config)
	demandLevel := getDemandLevel(ratio, s.config)

	s.multipliers[zoneID] = &SurgeMultiplier{
		ZoneID:      zoneID,
		Multiplier:  multiplier,
		DemandLevel: demandLevel,
		ValidUntil:  time.Now().Add(time.Duration(s.config.UpdateIntervalSec) * time.Second),
	}
}

// GetMultiplier returns the current surge multiplier for a zone.
func (s *SurgeService) GetMultiplier(zoneID uuid.UUID) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if m, ok := s.multipliers[zoneID]; ok {
		if time.Now().Before(m.ValidUntil) {
			return m.Multiplier
		}
	}
	return 1.0
}

// CalculateFare calculates the final fare with surge.
func (s *SurgeService) CalculateFare(baseFareCents int64, zoneID uuid.UUID) int64 {
	multiplier := s.GetMultiplier(zoneID)
	return int64(math.Round(float64(baseFareCents) * multiplier))
}

// GetHeatmap returns demand heatmap data.
func (s *SurgeService) GetHeatmap() []DemandHeatmap {
	s.mu.RLock()
	defer s.mu.RUnlock()

	heatmap := make([]DemandHeatmap, 0, len(s.multipliers))
	for zoneID, m := range s.multipliers {
		heatmap = append(heatmap, DemandHeatmap{
			ZoneID:          zoneID,
			SurgeMultiplier: m.Multiplier,
			DemandTrend:     m.DemandLevel,
			UpdatedAt:       time.Now(),
		})
	}
	return heatmap
}

func calculateMultiplier(ratio float64, config SurgeConfig) float64 {
	switch {
	case ratio >= config.ThresholdExtreme:
		return config.MaxMultiplier
	case ratio >= config.ThresholdHigh:
		return config.MinMultiplier + (config.MaxMultiplier-config.MinMultiplier)*(ratio-config.ThresholdHigh)/(config.ThresholdExtreme-config.ThresholdHigh)
	case ratio >= config.ThresholdLow:
		return config.MinMultiplier + (ratio-config.ThresholdLow)/(config.ThresholdHigh-config.ThresholdLow)*0.5
	default:
		return config.MinMultiplier
	}
}

func getDemandLevel(ratio float64, config SurgeConfig) string {
	switch {
	case ratio >= config.ThresholdExtreme:
		return "extreme"
	case ratio >= config.ThresholdHigh:
		return "high"
	case ratio >= config.ThresholdLow:
		return "normal"
	default:
		return "low"
	}
}
