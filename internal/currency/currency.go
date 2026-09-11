package currency

import (
	"fmt"
	"math"
	"strings"
)

// AOA represents Angolan Kwanza currency
type AOA struct {
	// Amount represents the value in Kwanzas (could be cents for precise calculations)
	Amount int64

	// Precision indicates if Amount is in cents (true) or whole Kwanzas (false)
	Precision bool
}

// NewAOA creates a new AOA amount from whole Kwanzas.
// This is a convenience alias for NewAOAFromWhole.
func NewAOA(amount int64) AOA {
	return NewAOAFromWhole(amount)
}

// NewAOAFromWhole creates a new AOA amount from whole Kwanzas
func NewAOAFromWhole(amount int64) AOA {
	return AOA{
		Amount:    amount,
		Precision: false,
	}
}

// NewAOAFromCents creates a new AOA amount from cents (for precise calculations)
func NewAOAFromCents(cents int64) AOA {
	return AOA{
		Amount:    cents,
		Precision: true,
	}
}

// NewAOAFromFloat creates a new AOA amount from a float value
func NewAOAFromFloat(amount float64) AOA {
	// Convert to cents for precision
	cents := int64(math.Round(amount * 100))
	return AOA{
		Amount:    cents,
		Precision: true,
	}
}

// WholeKwanzas returns the amount in whole Kwanzas
func (a AOA) WholeKwanzas() int64 {
	if a.Precision {
		return a.Amount / 100
	}
	return a.Amount
}

// Cents returns the amount in cents
func (a AOA) Cents() int64 {
	if !a.Precision {
		return a.Amount * 100
	}
	return a.Amount
}

// Float returns the amount as a float64
func (a AOA) Float() float64 {
	if a.Precision {
		return float64(a.Amount) / 100.0
	}
	return float64(a.Amount)
}

// Add adds another AOA amount to this one
func (a AOA) Add(other AOA) AOA {
	// Convert both to cents for calculation
	cents := a.Cents() + other.Cents()
	return NewAOAFromCents(cents)
}

// Subtract subtracts another AOA amount from this one
func (a AOA) Subtract(other AOA) AOA {
	cents := a.Cents() - other.Cents()
	return NewAOAFromCents(cents)
}

// Multiply multiplies the amount by a factor
func (a AOA) Multiply(factor float64) AOA {
	cents := int64(math.Round(float64(a.Cents()) * factor))
	return NewAOAFromCents(cents)
}

// Divide divides the amount by a divisor
func (a AOA) Divide(divisor float64) AOA {
	if divisor == 0 {
		return NewAOAFromCents(0)
	}
	cents := int64(math.Round(float64(a.Cents()) / divisor))
	return NewAOAFromCents(cents)
}

// Percentage calculates a percentage of the amount
func (a AOA) Percentage(percent float64) AOA {
	return a.Multiply(percent / 100.0)
}

// IsNegative checks if the amount is negative
func (a AOA) IsNegative() bool {
	return a.Amount < 0
}

// IsZero checks if the amount is zero
func (a AOA) IsZero() bool {
	return a.Amount == 0
}

// FormatShort formats the amount as "Kz 1.500" or "Kz 1.500,00"
func (a AOA) FormatShort(decimals bool) string {
	if decimals {
		return fmt.Sprintf("Kz %s", a.formatWithDecimals())
	}
	return fmt.Sprintf("Kz %s", a.formatWithoutDecimals())
}

// FormatCompact formats large amounts compactly: "Kz 1.500" or "Kz 1.500,00"
func (a AOA) FormatCompact() string {
	return a.FormatShort(true)
}

// FormatFull formats with full detail: "1.500,00 Kz"
func (a AOA) FormatFull() string {
	return fmt.Sprintf("%s Kz", a.formatWithDecimals())
}

// FormatForReceipt formats for receipts: "Kz 1.500,00"
func (a AOA) FormatForReceipt() string {
	return a.FormatShort(true)
}

// FormatForDisplay formats for UI display: "1.500 Kz" or "1.500,00 Kz"
func (a AOA) FormatForDisplay(showDecimals bool) string {
	if showDecimals {
		return fmt.Sprintf("%s Kz", a.formatWithDecimals())
	}
	return fmt.Sprintf("%s Kz", a.formatWithoutDecimals())
}

// formatWithDecimals formats with decimal places: "1.500,00"
func (a AOA) formatWithDecimals() string {
	value := a.Float()

	// Format integer part with separators
	formattedInt := formatInteger(int64(math.Abs(value)))

	// Get decimal part
	decimalPart := int64(math.Abs(value)*100) % 100

	// Handle sign
	sign := ""
	if value < 0 {
		sign = "-"
	}

	return fmt.Sprintf("%s%s,%02d", sign, formattedInt, decimalPart)
}

// formatWithoutDecimals formats without decimal places: "1.500"
func (a AOA) formatWithoutDecimals() string {
	value := a.Float()

	// Format integer part with separators
	formattedInt := formatInteger(int64(math.Abs(value)))

	// Handle sign
	sign := ""
	if value < 0 {
		sign = "-"
	}

	return fmt.Sprintf("%s%s", sign, formattedInt)
}

// formatInteger formats an integer with thousand separators (1.500.000)
func formatInteger(n int64) string {
	s := fmt.Sprintf("%d", n)

	if len(s) <= 3 {
		return s
	}

	var parts []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{s[start:i]}, parts...)
	}

	return strings.Join(parts, ".")
}

// ParseAOA parses a string like "1.500,00" or "Kz 1.500" into an AOA amount
func ParseAOA(input string) (AOA, error) {
	// Remove common prefixes and whitespace
	clean := strings.TrimSpace(input)
	clean = strings.ReplaceAll(clean, "Kz", "")
	clean = strings.ReplaceAll(clean, "k", "")
	clean = strings.ReplaceAll(clean, " ", "")

	// Check for decimal separator
	hasDecimals := strings.Contains(clean, ",")

	// Remove thousand separators
	clean = strings.ReplaceAll(clean, ".", "")

	if hasDecimals {
		// Remove decimal separator but track position
		parts := strings.Split(clean, ",")
		if len(parts) == 2 {
			// Rebuild as integer with 2 decimal places
			wholePart, err1 := parseInt64(parts[0])
			decimalPart, err2 := parseInt64(parts[1])
			if err1 != nil || err2 != nil {
				return AOA{}, fmt.Errorf("invalid AOA amount: %s", input)
			}
			// Pad decimal part to 2 digits
			for len(parts[1]) < 2 {
				parts[1] = parts[1] + "0"
			}
			decimalPart, _ = parseInt64(parts[1])
			amount := wholePart*100 + decimalPart
			return NewAOAFromCents(amount), nil
		}
	}

	// No decimals, parse as whole kwanzas
	amount, err := parseInt64(clean)
	if err != nil {
		return AOA{}, fmt.Errorf("invalid AOA amount: %s", input)
	}

	return NewAOAFromWhole(amount), nil
}

// parseInt64 safely parses a string to int64
func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// Zero returns a zero AOA amount
func Zero() AOA {
	return NewAOAFromWhole(0)
}

// Min returns the smaller of two amounts
func Min(a, b AOA) AOA {
	if a.Cents() < b.Cents() {
		return a
	}
	return b
}

// Max returns the larger of two amounts
func Max(a, b AOA) AOA {
	if a.Cents() > b.Cents() {
		return a
	}
	return b
}
