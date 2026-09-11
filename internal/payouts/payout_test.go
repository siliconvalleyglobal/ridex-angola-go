package payouts

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/currency"
)

func TestPayoutService_CreatePayout(t *testing.T) {
	service := NewPayoutService()

	tests := []struct {
		name    string
		request CreatePayoutInput
		wantErr bool
	}{
		{
			name: "valid payout request",
			request: CreatePayoutInput{
				DriverID: uuid.New(),
				Amount:   currency.NewAOA(10000),
				Method:   PayoutMethodMulticaixa,
			},
			wantErr: false,
		},
		{
			name: "missing driver ID",
			request: CreatePayoutInput{
				Amount: currency.NewAOA(10000),
				Method: PayoutMethodMulticaixa,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			request: CreatePayoutInput{
				DriverID: uuid.New(),
				Amount:   currency.NewAOA(0),
				Method:   PayoutMethodMulticaixa,
			},
			wantErr: true,
		},
		{
			name: "missing method",
			request: CreatePayoutInput{
				DriverID: uuid.New(),
				Amount:   currency.NewAOA(10000),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payout, err := service.CreatePayout(tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePayout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if payout.ID == uuid.Nil {
					t.Error("CreatePayout() payout ID should not be nil")
				}
				if payout.Status != PayoutStatusPending {
					t.Errorf("CreatePayout() status = %v, want %v", payout.Status, PayoutStatusPending)
				}
			}
		})
	}
}

func TestPayoutService_ProcessPayout(t *testing.T) {
	service := NewPayoutService()

	payout := &Payout{
		ID:     uuid.New(),
		Amount: currency.NewAOA(10000),
		Status: PayoutStatusPending,
	}

	err := service.ProcessPayout(payout)
	if err != nil {
		t.Errorf("ProcessPayout() error = %v", err)
	}

	if payout.Status != PayoutStatusCompleted {
		t.Errorf("ProcessPayout() status = %v, want %v", payout.Status, PayoutStatusCompleted)
	}
}

func TestPayoutService_CancelPayout(t *testing.T) {
	service := NewPayoutService()

	payout := &Payout{
		ID:     uuid.New(),
		Amount: currency.NewAOA(10000),
		Status: PayoutStatusPending,
	}

	err := service.CancelPayout(payout)
	if err != nil {
		t.Errorf("CancelPayout() error = %v", err)
	}

	if payout.Status != PayoutStatusCancelled {
		t.Errorf("CancelPayout() status = %v, want %v", payout.Status, PayoutStatusCancelled)
	}
}

func TestCalculateCommission(t *testing.T) {
	amount := currency.NewAOA(10000)
	commissionRate := 20.0

	commission := CalculateCommission(amount, commissionRate)
	expected := currency.NewAOA(2000)

	if commission.Cents() != expected.Cents() {
		t.Errorf("CalculateCommission() = %v, want %v", commission.Cents(), expected.Cents())
	}
}

func TestCalculateDriverEarnings(t *testing.T) {
	amount := currency.NewAOA(10000)
	commissionRate := 20.0

	earnings := CalculateDriverEarnings(amount, commissionRate)
	expected := currency.NewAOA(8000)

	if earnings.Cents() != expected.Cents() {
		t.Errorf("CalculateDriverEarnings() = %v, want %v", earnings.Cents(), expected.Cents())
	}
}

func TestValidatePayoutMethod(t *testing.T) {
	tests := []struct {
		method  PayoutMethod
		wantErr bool
	}{
		{PayoutMethodMulticaixa, false},
		{PayoutMethodMulticada, false},
		{PayoutMethodMobileMoney, false},
		{PayoutMethodBank, false},
		{PayoutMethodBankTransfer, false},
		{PayoutMethodCash, false},
		{"invalid_method", true},
	}

	for _, tt := range tests {
		err := ValidatePayoutMethod(tt.method)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePayoutMethod(%s) error = %v, wantErr %v", tt.method, err, tt.wantErr)
		}
	}
}

func TestDefaultCommissionRate(t *testing.T) {
	rate := DefaultCommissionRate()
	if rate != 20.0 {
		t.Errorf("DefaultCommissionRate() = %v, want 20.0", rate)
	}
}
