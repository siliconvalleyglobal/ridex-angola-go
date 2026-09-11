package drivers

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// DriverScore represents a driver's performance score.
type DriverScore struct {
	DriverID         uuid.UUID
	OverallScore     float64
	AcceptanceRate   float64
	CompletionRate   float64
	AvgResponseSec   float64
	AvgRating        float64
	PunctualityScore float64
	SafetyScore      float64
	TotalRides       int
	CompletedRides   int
	CancelledRides   int
	NoShows          int
	LastCalculatedAt time.Time
}

// ScoreWeights defines how each factor contributes to the overall score.
type ScoreWeights struct {
	AcceptanceWeight  float64
	CompletionWeight  float64
	RatingWeight      float64
	ResponseWeight    float64
	PunctualityWeight float64
	SafetyWeight      float64
}

// DefaultScoreWeights returns the default scoring weights.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		AcceptanceWeight:  0.20,
		CompletionWeight:  0.25,
		RatingWeight:      0.25,
		ResponseWeight:    0.10,
		PunctualityWeight: 0.10,
		SafetyWeight:      0.10,
	}
}

// DriverTier represents a driver's incentive tier.
type DriverTier string

const (
	TierBronze   DriverTier = "bronze"
	TierSilver   DriverTier = "silver"
	TierGold     DriverTier = "gold"
	TierPlatinum DriverTier = "platinum"
)

// TierThresholds defines the minimum score for each tier.
type TierThresholds struct {
	SilverMin   float64
	GoldMin     float64
	PlatinumMin float64
}

// DefaultTierThresholds returns default tier thresholds.
func DefaultTierThresholds() TierThresholds {
	return TierThresholds{
		SilverMin:   70.0,
		GoldMin:     85.0,
		PlatinumMin: 95.0,
	}
}

// DriverMetrics contains the raw metrics used for driver scoring.
type DriverMetrics struct {
	DriverID           uuid.UUID
	TotalRides         int64
	CompletedRides     int64
	CancelledRides     int64
	NoShows            int64
	TotalOffers        int64
	AcceptedOffers     int64
	AvgRating          float64
	AvgResponseTimeSec int64
}

// CalculateDriverScore computes the overall driver performance score.
func CalculateDriverScore(
	metrics DriverMetrics,
	weights ScoreWeights,
) DriverScore {
	score := DriverScore{
		DriverID:       metrics.DriverID,
		TotalRides:     int(metrics.TotalRides),
		CompletedRides: int(metrics.CompletedRides),
		CancelledRides: int(metrics.CancelledRides),
		NoShows:        int(metrics.NoShows),
	}

	// Calculate acceptance rate (0-100)
	if metrics.TotalOffers > 0 {
		score.AcceptanceRate = float64(metrics.AcceptedOffers) / float64(metrics.TotalOffers) * 100
	}

	// Calculate completion rate (0-100)
	if metrics.TotalRides > 0 {
		score.CompletionRate = float64(metrics.CompletedRides) / float64(metrics.TotalRides) * 100
	}

	// Average rating (already 0-5 scale, convert to 0-100)
	score.AvgRating = float64(metrics.AvgRating) * 20

	// Response time score (faster = higher score, max 120 seconds)
	if metrics.AvgResponseTimeSec > 0 {
		score.AvgResponseSec = float64(metrics.AvgResponseTimeSec)
		responseScore := math.Max(0, 100-(float64(metrics.AvgResponseTimeSec)/120)*100)
		_ = responseScore // used in weighted calculation
	}

	// Calculate overall weighted score
	score.OverallScore = score.AcceptanceRate*weights.AcceptanceWeight +
		score.CompletionRate*weights.CompletionWeight +
		score.AvgRating*weights.RatingWeight +
		score.PunctualityScore*weights.PunctualityWeight +
		score.SafetyScore*weights.SafetyWeight

	score.LastCalculatedAt = time.Now().UTC()
	return score
}

// DetermineTier returns the driver tier based on overall score.
func DetermineTier(score float64, thresholds TierThresholds) DriverTier {
	switch {
	case score >= thresholds.PlatinumMin:
		return TierPlatinum
	case score >= thresholds.GoldMin:
		return TierGold
	case score >= thresholds.SilverMin:
		return TierSilver
	default:
		return TierBronze
	}
}

// TierBenefits defines the benefits for each tier.
type TierBenefits struct {
	Tier              DriverTier
	PriorityMatching  bool
	BonusMultiplier   float64 // earnings bonus
	MinAcceptanceRate float64 // to maintain tier
	MaxCancellations  int     // per week
	Perks             []string
}

// GetTierBenefits returns benefits for a given tier.
func GetTierBenefits(tier DriverTier) TierBenefits {
	switch tier {
	case TierPlatinum:
		return TierBenefits{
			Tier:              tier,
			PriorityMatching:  true,
			BonusMultiplier:   1.10, // 10% bonus
			MinAcceptanceRate: 90,
			MaxCancellations:  1,
			Perks:             []string{"priority_support", "instant_payout", "exclusive_badges"},
		}
	case TierGold:
		return TierBenefits{
			Tier:              tier,
			PriorityMatching:  true,
			BonusMultiplier:   1.05, // 5% bonus
			MinAcceptanceRate: 80,
			MaxCancellations:  2,
			Perks:             []string{"priority_support", "faster_payout"},
		}
	case TierSilver:
		return TierBenefits{
			Tier:              tier,
			PriorityMatching:  false,
			BonusMultiplier:   1.02, // 2% bonus
			MinAcceptanceRate: 70,
			MaxCancellations:  3,
			Perks:             []string{"weekly_bonus"},
		}
	default:
		return TierBenefits{
			Tier:              TierBronze,
			PriorityMatching:  false,
			BonusMultiplier:   1.00,
			MinAcceptanceRate: 50,
			MaxCancellations:  5,
			Perks:             []string{},
		}
	}
}

// PerformanceReport contains weekly/monthly performance data.
type PerformanceReport struct {
	DriverID       uuid.UUID
	Period         string // "weekly", "monthly"
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Score          DriverScore
	Tier           DriverTier
	RidesCompleted int
	EarningsCents  int64
	TargetRides    int
	Achievements   []string
}
