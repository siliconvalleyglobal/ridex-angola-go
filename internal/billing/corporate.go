package billing

import (
	"time"

	"github.com/google/uuid"
)

// CorporateInvoice represents a monthly invoice for a business account.
type CorporateInvoice struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	RideCount   int
	TotalCents  int64
	TaxCents    int64
	GrandTotal  int64
	Status      string // draft, sent, paid, overdue
	DueDate     time.Time
	SentAt      *time.Time
	PaidAt      *time.Time
	CreatedAt   time.Time
}

// CostCenter represents a department or project within a corporate account.
type CostCenter struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	Name        string
	Code        string
	BudgetCents int64
	SpentCents  int64
	Active      bool
	CreatedAt   time.Time
}

// BillingCycle represents the billing period configuration.
type BillingCycle struct {
	AccountID     uuid.UUID
	Frequency     string // monthly, quarterly
	DayOfMonth    int
	NextBillingAt time.Time
	AutoInvoice   bool
}

// InvoiceItem represents a line item on a corporate invoice.
type InvoiceItem struct {
	Description  string
	RideID       uuid.UUID
	Date         time.Time
	AmountCents  int64
	CostCenterID *uuid.UUID
}

// BillingService handles corporate billing operations.
type BillingService struct{}

// NewBillingService creates a new billing service.
func NewBillingService() *BillingService {
	return &BillingService{}
}

// GenerateInvoice creates a consolidated invoice for a billing period.
func (s *BillingService) GenerateInvoice(accountID uuid.UUID, rides []uuid.UUID, totalCents int64) *CorporateInvoice {
	taxRate := 0.14 // 14% IVA in Angola
	taxCents := int64(float64(totalCents) * taxRate)

	return &CorporateInvoice{
		ID:         uuid.New(),
		AccountID:  accountID,
		RideCount:  len(rides),
		TotalCents: totalCents,
		TaxCents:   taxCents,
		GrandTotal: totalCents + taxCents,
		Status:     "draft",
		DueDate:    time.Now().AddDate(0, 0, 30),
		CreatedAt:  time.Now(),
	}
}

// CalculateBudgetUtilization returns the percentage of budget used.
func CalculateBudgetUtilization(spentCents, budgetCents int64) float64 {
	if budgetCents == 0 {
		return 0
	}
	return float64(spentCents) / float64(budgetCents) * 100
}

// IsOverBudget checks if spending exceeds budget.
func IsOverBudget(spentCents, budgetCents int64) bool {
	return spentCents >= budgetCents
}
