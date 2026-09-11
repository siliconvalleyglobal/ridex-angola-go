package currency

import (
	"testing"
)

func TestAOA_FormatShort(t *testing.T) {
	tests := []struct {
		name           string
		amount         int64
		precision      bool
		decimals       bool
		expectedFormat string
	}{
		{
			name:           "Whole kwanzas without decimals",
			amount:         1500,
			precision:      false,
			decimals:       false,
			expectedFormat: "Kz 1.500",
		},
		{
			name:           "Whole kwanzas with decimals flag",
			amount:         1500,
			precision:      false,
			decimals:       true,
			expectedFormat: "Kz 1.500,00",
		},
		{
			name:           "Cents converted to kwanzas with decimals",
			amount:         150000, // 1.500 kwanzas in cents
			precision:      true,
			decimals:       true,
			expectedFormat: "Kz 1.500,00",
		},
		{
			name:           "Large amount",
			amount:         1500000,
			precision:      false,
			decimals:       false,
			expectedFormat: "Kz 1.500.000",
		},
		{
			name:           "Zero amount",
			amount:         0,
			precision:      false,
			decimals:       false,
			expectedFormat: "Kz 0",
		},
		{
			name:           "Negative amount",
			amount:         -1500,
			precision:      false,
			decimals:       false,
			expectedFormat: "Kz -1.500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ao := AOA{Amount: tt.amount, Precision: tt.precision}
			result := ao.FormatShort(tt.decimals)
			if result != tt.expectedFormat {
				t.Errorf("expected %s, got %s", tt.expectedFormat, result)
			}
		})
	}
}

func TestAOA_FormatFull(t *testing.T) {
	ao := NewAOAFromWhole(1500)
	result := ao.FormatFull()

	if result != "1.500,00 Kz" {
		t.Errorf("expected '1.500,00 Kz', got '%s'", result)
	}
}

func TestAOA_Operations(t *testing.T) {
	a := NewAOAFromWhole(1500)
	b := NewAOAFromWhole(500)

	// Test Add
	sum := a.Add(b)
	if sum.WholeKwanzas() != 2000 {
		t.Errorf("expected 2000, got %d", sum.WholeKwanzas())
	}

	// Test Subtract
	diff := a.Subtract(b)
	if diff.WholeKwanzas() != 1000 {
		t.Errorf("expected 1000, got %d", diff.WholeKwanzas())
	}

	// Test Multiply
	doubled := a.Multiply(2)
	if doubled.WholeKwanzas() != 3000 {
		t.Errorf("expected 3000, got %d", doubled.WholeKwanzas())
	}

	// Test Divide
	quarter := a.Divide(2)
	if quarter.WholeKwanzas() != 750 {
		t.Errorf("expected 750, got %d", quarter.WholeKwanzas())
	}

	// Test Percentage
	half := a.Percentage(50)
	if half.WholeKwanzas() != 750 {
		t.Errorf("expected 750, got %d", half.WholeKwanzas())
	}
}

func TestAOA_IsNegative(t *testing.T) {
	positive := NewAOAFromWhole(100)
	negative := NewAOAFromWhole(-100)
	zero := NewAOAFromWhole(0)

	if positive.IsNegative() {
		t.Error("positive amount should not be negative")
	}
	if !negative.IsNegative() {
		t.Error("negative amount should be negative")
	}
	if zero.IsNegative() {
		t.Error("zero should not be negative")
	}
}

func TestAOA_IsZero(t *testing.T) {
	zero := NewAOAFromWhole(0)
	nonZero := NewAOAFromWhole(100)

	if !zero.IsZero() {
		t.Error("zero should be zero")
	}
	if nonZero.IsZero() {
		t.Error("non-zero should not be zero")
	}
}

func TestNewAOAFromFloat(t *testing.T) {
	ao := NewAOAFromFloat(1500.50)

	if ao.WholeKwanzas() != 1500 {
		t.Errorf("expected 1500, got %d", ao.WholeKwanzas())
	}
	if ao.Cents() != 150050 {
		t.Errorf("expected 150050, got %d", ao.Cents())
	}
}

func TestMinMax(t *testing.T) {
	a := NewAOAFromWhole(100)
	b := NewAOAFromWhole(200)

	if Min(a, b).WholeKwanzas() != 100 {
		t.Error("Min should return 100")
	}
	if Max(a, b).WholeKwanzas() != 200 {
		t.Error("Max should return 200")
	}
}

func TestZero(t *testing.T) {
	z := Zero()
	if !z.IsZero() {
		t.Error("Zero() should return zero amount")
	}
}
