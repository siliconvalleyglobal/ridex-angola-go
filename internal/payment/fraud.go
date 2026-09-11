package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ridex/ridex-angola/internal/db"
)

// Fraud evaluation rejects a charge before any provider money is reserved.
// A failure to read the signals itself fails closed: a charge is only created
// when the system could prove the action is safe. This mirrors the payment
// pipeline's uncertainty-aware semantics — when the system cannot prove the
// action is safe, it does not act.
var (
	ErrFraudVelocityExceeded = errors.New("too many payment intents recently")
	ErrFraudDuplicateAmount  = errors.New("too many identical payment amounts recently")
)

// FraudStore is the signal read surface for the fraud evaluator.
type FraudStore interface {
	GetRiderChargeSignals(context.Context, db.GetRiderChargeSignalsParams) (db.GetRiderChargeSignalsRow, error)
}

// FraudConfig bounds the two rejection rules. Windows are evaluated against
// the moment of evaluation; limits are inclusive charge counts inside the
// window (a charge is rejected when the count is >= the limit).
type FraudConfig struct {
	VelocityWindow  time.Duration
	VelocityLimit   int64
	DuplicateWindow time.Duration
	DuplicateLimit  int64
}

// DefaultFraudConfig is the ops baseline: a rider may not create 10 intents
// inside an hour, and may not create 2 identical-amount charges inside 30
// minutes. A rider legitimately requests at most a handful of rides per hour
// and fares are rarely byte-identical that close together.
func DefaultFraudConfig() FraudConfig {
	return FraudConfig{
		VelocityWindow:  time.Hour,
		VelocityLimit:   10,
		DuplicateWindow: 30 * time.Minute,
		DuplicateLimit:  2,
	}
}

// Fraud gates charge creation on velocity and duplicate-amount signals.
type Fraud struct {
	store  FraudStore
	config FraudConfig
}

// NewFraud builds the evaluator. The config must be valid (positive windows
// and limits); the caller is expected to derive it from validated config.
func NewFraud(store FraudStore, config FraudConfig) (*Fraud, error) {
	if store == nil {
		return nil, fmt.Errorf("fraud store is required")
	}
	if config.VelocityWindow <= 0 || config.VelocityLimit < 1 ||
		config.DuplicateWindow <= 0 || config.DuplicateLimit < 1 {
		return nil, fmt.Errorf("fraud windows and limits must be positive")
	}
	return &Fraud{store: store, config: config}, nil
}

// ChargeSignals is the evaluated signal set for one decision.
type ChargeSignals struct {
	RecentCharges    int64
	DuplicateAmounts int64
}

// Evaluate probes the rider's signals and rejects the charge when a rule
// fires. Both signals are computed in a single scan; the scan bound is the
// wider of the two windows so both sub-windows are covered.
func (f *Fraud) Evaluate(ctx context.Context, riderID uuid.UUID, amountCents int64) (ChargeSignals, error) {
	now := time.Now().UTC()
	velocitySince := now.Add(-f.config.VelocityWindow)
	duplicateSince := now.Add(-f.config.DuplicateWindow)
	scanSince := velocitySince
	if duplicateSince.Before(scanSince) {
		scanSince = duplicateSince
	}
	row, err := f.store.GetRiderChargeSignals(ctx, db.GetRiderChargeSignalsParams{
		RiderID:     riderID,
		CreatedAt:   pgtype.Timestamptz{Time: scanSince, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: velocitySince, Valid: true},
		CreatedAt_3: pgtype.Timestamptz{Time: duplicateSince, Valid: true},
		AmountCents: pgtype.Numeric{Int: big.NewInt(amountCents), Valid: true},
	})
	if err != nil {
		return ChargeSignals{}, fmt.Errorf("read fraud signals: %w", err)
	}
	signals := ChargeSignals{RecentCharges: row.RecentCharges, DuplicateAmounts: row.DuplicateAmounts}
	if signals.RecentCharges >= f.config.VelocityLimit {
		return signals, ErrFraudVelocityExceeded
	}
	if signals.DuplicateAmounts >= f.config.DuplicateLimit {
		return signals, ErrFraudDuplicateAmount
	}
	return signals, nil
}
