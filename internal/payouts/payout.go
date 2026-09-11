package payouts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/currency"
)

// PayoutMethod represents the method for driver payouts in Angola.
// These values follow the payout_requests / payout_methods CHECK constraints
// in migration 000023 ('multicada' uses the constraint's spelling). The
// wallet-backed ledger registry in methods.go builds on these constants.
type PayoutMethod string

const (
	// PayoutMethodMobileMoney represents payout via mobile money (Africell Money, Unitel Money, etc.)
	PayoutMethodMobileMoney PayoutMethod = "mobile_money"

	// PayoutMethodBankTransfer is the legacy API spelling for bank payouts. It
	// normalizes to PayoutMethodBank ('bank') in the ledger registry.
	PayoutMethodBankTransfer PayoutMethod = "bank_transfer"

	// PayoutMethodCash represents cash payout
	PayoutMethodCash PayoutMethod = "cash"
)

// PayoutMethodBank is the canonical bank channel name used by the ledger
// registry and the payout_requests CHECK constraint ('bank').
const PayoutMethodBank PayoutMethod = "bank"

// PayoutMethodMulticaixa is the correctly-spelled Multicaixa channel. It
// normalizes to PayoutMethodMulticada ('multicada') in the ledger registry so
// writes satisfy the payout_requests CHECK constraint.
const PayoutMethodMulticaixa PayoutMethod = "multicaixa"

// PayoutMethodMulticada is the constraint-spelled Multicaixa value persisted
// to the database ('multicada').
const PayoutMethodMulticada PayoutMethod = "multicada"

// PayoutStatus represents the status of a payout
type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "pending"
	PayoutStatusProcessing PayoutStatus = "processing"
	PayoutStatusCompleted  PayoutStatus = "completed"
	PayoutStatusFailed     PayoutStatus = "failed"
	PayoutStatusCancelled  PayoutStatus = "cancelled"
	PayoutStatusRefunded   PayoutStatus = "refunded"
)

// Payout represents a driver payout
type Payout struct {
	ID          uuid.UUID      `json:"id"`
	DriverID    uuid.UUID      `json:"driverId"`
	Amount      currency.AOA   `json:"amount"`
	Method      PayoutMethod   `json:"method"`
	Status      PayoutStatus   `json:"status"`
	ReferenceID string         `json:"referenceId,omitempty"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	ProcessedAt *time.Time     `json:"processedAt,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// PayoutRequest represents a request to create a payout.
// It is the simulated-service shape; the wallet-backed ledger returns
// db.PayoutRequest rows instead.
type CreatePayoutInput struct {
	DriverID    uuid.UUID      `json:"driverId"`
	Amount      currency.AOA   `json:"amount"`
	Method      PayoutMethod   `json:"method"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// PayoutResponse represents the response from creating a payout
type PayoutResponse struct {
	PayoutID  uuid.UUID    `json:"payoutId"`
	Status    PayoutStatus `json:"status"`
	Message   string       `json:"message"`
	Reference string       `json:"reference,omitempty"`
	// For Multicaixa: the location where the driver can withdraw
	Locations []MulticaixaLocation `json:"locations,omitempty"`
}

// MulticaixaLocation represents a Multicaixa ATM location
type MulticaixaLocation struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Address       string  `json:"address"`
	City          string  `json:"city"`
	HasATMs       bool    `json:"hasAtms"`
	HasMulticaixa bool    `json:"hasMulticaixa"`
	Distance      float64 `json:"distance,omitempty"`
}

// PayoutService handles driver payouts
type PayoutService struct {
	// In production, this would be connected to actual payout providers
	// For now, it's a simulation
}

// NewPayoutService creates a new payout service
func NewPayoutService() *PayoutService {
	return &PayoutService{}
}

// CreatePayout creates a new payout request
func (s *PayoutService) CreatePayout(req CreatePayoutInput) (*Payout, error) {
	if req.DriverID == uuid.Nil {
		return nil, errors.New("driver ID is required")
	}

	if req.Amount.IsZero() || req.Amount.IsNegative() {
		return nil, errors.New("amount must be positive")
	}

	if req.Method == "" {
		return nil, errors.New("payout method is required")
	}

	payout := &Payout{
		ID:          uuid.New(),
		DriverID:    req.DriverID,
		Amount:      req.Amount,
		Method:      req.Method,
		Status:      PayoutStatusPending,
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
		Metadata:    req.Metadata,
	}

	// In production, this would initiate the actual payout with the provider
	// For now, we just create the payout record

	return payout, nil
}

// GetPayout returns a payout by ID
func (s *PayoutService) GetPayout(id uuid.UUID) (*Payout, error) {
	// In production, this would fetch from the database
	return nil, fmt.Errorf("payout %s not found", id.String())
}

// GetPayoutsByDriver returns all payouts for a driver
func (s *PayoutService) GetPayoutsByDriver(driverID uuid.UUID) ([]Payout, error) {
	// In production, this would fetch from the database
	return []Payout{}, nil
}

// ProcessPayout processes a payout (called by background worker)
func (s *PayoutService) ProcessPayout(payout *Payout) error {
	if payout.Status != PayoutStatusPending {
		return errors.New("payout is not in pending status")
	}

	// Update status to processing
	payout.Status = PayoutStatusProcessing

	// In production, this would call the actual payout provider
	// For now, we simulate success

	timeNow := time.Now().UTC()
	payout.Status = PayoutStatusCompleted
	payout.ProcessedAt = &timeNow

	return nil
}

// CancelPayout cancels a pending payout
func (s *PayoutService) CancelPayout(payout *Payout) error {
	if payout.Status != PayoutStatusPending {
		return errors.New("only pending payouts can be cancelled")
	}

	payout.Status = PayoutStatusCancelled
	return nil
}

// CalculateCommission calculates the commission for a payout
// In Angola, ride-hailing platforms typically take 15-25% commission
func CalculateCommission(payoutAmount currency.AOA, commissionRate float64) currency.AOA {
	return payoutAmount.Percentage(commissionRate)
}

// CalculateDriverEarnings calculates the driver's earnings after commission
func CalculateDriverEarnings(payoutAmount currency.AOA, commissionRate float64) currency.AOA {
	commission := CalculateCommission(payoutAmount, commissionRate)
	return payoutAmount.Subtract(commission)
}

// DefaultCommissionRate returns the default commission rate (20%)
func DefaultCommissionRate() float64 {
	return 20.0
}

// ValidatePayoutMethod validates if a payout method is supported.
// It accepts both the ledger ('bank', 'multicada') and legacy ('bank_transfer')
// spellings so older clients keep working.
func ValidatePayoutMethod(method PayoutMethod) error {
	switch PayoutMethod(strings.ToLower(strings.TrimSpace(string(method)))) {
	case PayoutMethodBank, PayoutMethodMobileMoney, PayoutMethodMulticada, PayoutMethodMulticaixa,
		PayoutMethodCash, PayoutMethodBankTransfer:
		return nil
	default:
		return fmt.Errorf("unsupported payout method: %s", method)
	}
}

// ToJSON converts a payout to JSON string
func (p *Payout) ToJSON() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
