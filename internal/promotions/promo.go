package promotions

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// PromoCode represents a promotional discount code.
type PromoCode struct {
	ID               uuid.UUID
	Code             string
	DiscountType     string
	DiscountCents    int64
	MaxUses          int
	CurrentUses      int
	ValidFrom        time.Time
	ValidUntil       time.Time
	MinRideCents     int64
	MaxDiscountCents int64
	IsActive         bool
	CreatedAt        time.Time
}

// PromoCodeResult represents the result of applying a promo code.
type PromoCodeResult struct {
	Valid          bool
	DiscountCents  int64
	FinalFareCents int64
	Message        string
}

// LoyaltyTier represents a loyalty program tier.
type LoyaltyTier string

const (
	LoyaltyTierBronze   LoyaltyTier = "bronze"
	LoyaltyTierSilver   LoyaltyTier = "silver"
	LoyaltyTierGold     LoyaltyTier = "gold"
	LoyaltyTierPlatinum LoyaltyTier = "platinum"
)

// TierThresholds defines minimum points for each tier.
type TierThresholds struct {
	SilverMin   int64
	GoldMin     int64
	PlatinumMin int64
}

// DefaultTierThresholds returns default tier thresholds.
func DefaultTierThresholds() TierThresholds {
	return TierThresholds{
		SilverMin:   1000,
		GoldMin:     5000,
		PlatinumMin: 10000,
	}
}

// PromoService handles promo code and loyalty operations.
type PromoService struct {
	q    *db.Queries
	dbtx pgx.Tx
}

// NewPromoService creates a new promo service.
func NewPromoService(q *db.Queries, dbtx pgx.Tx) *PromoService {
	return &PromoService{q: q, dbtx: dbtx}
}

// ValidatePromoCode checks if a promo code is valid and calculates discount.
func (s *PromoService) ValidatePromoCode(ctx context.Context, code string, fareCents int64, userID uuid.UUID) (PromoCodeResult, error) {
	promo, err := s.q.GetPromoCodeByCode(ctx, code)
	if err != nil {
		return PromoCodeResult{Valid: false, Message: "invalid promo code"}, nil
	}

	if !promo.IsActive {
		return PromoCodeResult{Valid: false, Message: "promo code is not active"}, nil
	}

	now := time.Now()
	if promo.ValidFrom.Valid && now.Before(promo.ValidFrom.Time) {
		return PromoCodeResult{Valid: false, Message: "promo code is not yet valid"}, nil
	}
	if promo.ValidUntil.Valid && now.After(promo.ValidUntil.Time) {
		return PromoCodeResult{Valid: false, Message: "promo code has expired"}, nil
	}

	if int(promo.CurrentUses) >= int(promo.MaxUses) {
		return PromoCodeResult{Valid: false, Message: "promo code usage limit reached"}, nil
	}

	if fareCents < promo.MinRideCents {
		return PromoCodeResult{Valid: false, Message: "ride amount below minimum"}, nil
	}

	used, err := s.q.CheckUserPromoUsage(ctx, db.CheckUserPromoUsageParams{
		PromoID: promo.ID,
		UserID:  userID,
	})
	if err != nil {
		return PromoCodeResult{Valid: false, Message: "error checking promo usage"}, err
	}
	if used {
		return PromoCodeResult{Valid: false, Message: "promo code already used"}, nil
	}

	var discountCents int64
	switch promo.DiscountType {
	case "percentage":
		discountCents = fareCents * promo.DiscountCents / 100
		if promo.MaxDiscountCents.Valid && discountCents > promo.MaxDiscountCents.Int64 {
			discountCents = promo.MaxDiscountCents.Int64
		}
	case "fixed":
		discountCents = promo.DiscountCents
	default:
		return PromoCodeResult{Valid: false, Message: "invalid discount type"}, nil
	}

	finalFare := fareCents - discountCents
	if finalFare < 0 {
		finalFare = 0
	}

	return PromoCodeResult{
		Valid:          true,
		DiscountCents:  discountCents,
		FinalFareCents: finalFare,
		Message:        "promo code applied",
	}, nil
}

// ApplyPromoCode applies a promo code to a ride.
func (s *PromoService) ApplyPromoCode(ctx context.Context, promoID, userID, rideID uuid.UUID) error {
	tx, err := s.dbtx.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txQ := s.q.WithTx(tx)
	if _, err := txQ.IncrementPromoUsage(ctx, promoID); err != nil {
		return err
	}

	err = txQ.RecordPromoUsage(ctx, db.RecordPromoUsageParams{
		PromoID:       promoID,
		UserID:        userID,
		RideID:        pgtype.UUID{Bytes: rideID, Valid: true},
		DiscountCents: 0,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CalculateRidePoints calculates points earned for completing a ride.
func CalculateRidePoints(fareCents int64, tier LoyaltyTier) int64 {
	basePoints := fareCents / 100

	var multiplier float64
	switch tier {
	case LoyaltyTierPlatinum:
		multiplier = 2.0
	case LoyaltyTierGold:
		multiplier = 1.5
	case LoyaltyTierSilver:
		multiplier = 1.25
	default:
		multiplier = 1.0
	}

	return int64(float64(basePoints) * multiplier)
}

// DetermineLoyaltyTier returns the tier based on total points.
func DetermineLoyaltyTier(totalPoints int64, thresholds TierThresholds) LoyaltyTier {
	switch {
	case totalPoints >= thresholds.PlatinumMin:
		return LoyaltyTierPlatinum
	case totalPoints >= thresholds.GoldMin:
		return LoyaltyTierGold
	case totalPoints >= thresholds.SilverMin:
		return LoyaltyTierSilver
	default:
		return LoyaltyTierBronze
	}
}
