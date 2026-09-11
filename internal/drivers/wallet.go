package drivers

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// Wallet represents a driver's earnings wallet.
type Wallet struct {
	DriverID         uuid.UUID
	BalanceCents     int64
	PendingCents     int64
	TotalEarnedCents int64
	TotalPaidCents   int64
	Currency         string
	LastPayoutAt     *time.Time
	UpdatedAt        time.Time
}

// WalletTransaction represents a single wallet transaction.
type WalletTransaction struct {
	ID            uuid.UUID
	DriverID      uuid.UUID
	Type          string
	AmountCents   int64
	BalanceAfter  int64
	ReferenceID   *uuid.UUID
	ReferenceType string
	Description   string
	Status        string
	CreatedAt     time.Time
}

// EarningBreakdown shows how earnings are calculated.
type EarningBreakdown struct {
	RideID          uuid.UUID
	BaseFareCents   int64
	DistanceCents   int64
	TimeCents       int64
	SurgeCents      int64
	BonusCents      int64
	TotalGrossCents int64
	CommissionCents int64
	NetEarningCents int64
	CommissionRate  float64
}

// PayoutRequest represents a driver's request for payout.
type PayoutRequest struct {
	ID            uuid.UUID
	DriverID      uuid.UUID
	AmountCents   int64
	Method        string
	Status        string
	ReferenceID   string
	RequestedAt   time.Time
	ProcessedAt   *time.Time
	FailureReason string
}

// WalletService handles wallet operations.
type WalletService struct {
	q *db.Queries
}

// NewWalletService creates a new wallet service.
func NewWalletService(q *db.Queries) *WalletService {
	return &WalletService{q: q}
}

// GetWallet retrieves a driver's wallet.
func (s *WalletService) GetWallet(ctx context.Context, driverID uuid.UUID) (*Wallet, error) {
	wallet, err := s.q.GetDriverWallet(ctx, driverID)
	if err != nil {
		return nil, err
	}
	return &Wallet{
		DriverID:         wallet.DriverID,
		BalanceCents:     wallet.BalanceCents,
		PendingCents:     wallet.PendingCents,
		TotalEarnedCents: wallet.TotalEarnedCents,
		TotalPaidCents:   wallet.TotalPaidCents,
		Currency:         wallet.Currency,
		UpdatedAt:        wallet.UpdatedAt.Time,
	}, nil
}

// AddEarning adds an earning to the driver's wallet.
func (s *WalletService) AddEarning(ctx context.Context, driverID uuid.UUID, breakdown EarningBreakdown) error {
	_, err := s.q.AddDriverEarning(ctx, db.AddDriverEarningParams{
		DriverID:     driverID,
		BalanceCents: breakdown.NetEarningCents,
	})
	if err != nil {
		return err
	}

	err = s.q.CreateWalletTransaction(ctx, db.CreateWalletTransactionParams{
		DriverID:      driverID,
		Type:          "earning",
		AmountCents:   breakdown.NetEarningCents,
		BalanceAfter:  0, // Will be calculated
		ReferenceID:   pgtype.UUID{Bytes: breakdown.RideID, Valid: true},
		ReferenceType: pgtype.Text{String: "ride", Valid: true},
		Description:   pgtype.Text{String: "Ride earning", Valid: true},
		Status:        "completed",
	})
	return err
}

// ProcessPayout processes a payout request.
func (s *WalletService) ProcessPayout(ctx context.Context, driverID uuid.UUID, amountCents int64, method string) error {
	if amountCents <= 0 {
		return errors.New("payout amount must be positive")
	}

	wallet, err := s.q.GetDriverWallet(ctx, driverID)
	if err != nil {
		return err
	}

	if wallet.BalanceCents < amountCents {
		return errors.New("insufficient balance")
	}

	_, err = s.q.DeductDriverBalance(ctx, db.DeductDriverBalanceParams{
		DriverID:     driverID,
		BalanceCents: amountCents,
	})
	if err != nil {
		return err
	}

	err = s.q.CreateWalletTransaction(ctx, db.CreateWalletTransactionParams{
		DriverID:      driverID,
		Type:          "payout",
		AmountCents:   -amountCents,
		BalanceAfter:  wallet.BalanceCents - amountCents,
		ReferenceType: pgtype.Text{String: "payout", Valid: true},
		Description:   pgtype.Text{String: "Payout to " + method, Valid: true},
		Status:        "completed",
	})
	return err
}

// CalculateCommission calculates platform commission.
func CalculateCommission(grossCents int64, commissionRate float64) int64 {
	rate := new(big.Float).SetFloat64(commissionRate / 100)
	amount := new(big.Float).SetInt64(grossCents)
	result := new(big.Float).Mul(amount, rate)
	commission, _ := result.Int64()
	return commission
}

// Helper to convert pgtype.Numeric to int64
func numericToInt64(n pgtype.Numeric) int64 {
	if !n.Valid || n.Int == nil {
		return 0
	}
	return n.Int.Int64() * int64(big.NewInt(1).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil).Int64())
}
