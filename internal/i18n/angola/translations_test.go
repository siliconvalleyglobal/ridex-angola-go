package angola

import (
	"testing"
)

func TestTranslator_T(t *testing.T) {
	tr := NewTranslator()

	// Test ride messages
	if tr.T(MsgRideRequested) != "Pedido de corrida recebido com sucesso." {
		t.Errorf("expected ride request message")
	}

	if tr.T(MsgDriverFound) != "Motorista encontrado. Agora vai para o seu encontro." {
		t.Errorf("expected driver found message")
	}

	// Test format string interpolation
	formatted := tr.T(MsgOTPVerificationCode, "123456")
	if formatted != "O seu código de verificação é: 123456" {
		t.Errorf("expected formatted OTP message, got: %s", formatted)
	}

	// Test payment formatting
	paymentMsg := tr.T(MsgPaymentAmountDue)
	if paymentMsg != "Valor a pagar: %s AOA" {
		t.Errorf("expected payment amount message")
	}

	// Test missing key fallback
	fallback := tr.T("nonexistent.key")
	if fallback != "[nonexistent.key]" {
		t.Errorf("expected fallback for missing key")
	}
}

func TestFormatCurrency(t *testing.T) {
	result := FormatCurrency(10000)
	if result != "100.00 AOA" {
		t.Errorf("expected 100.00 AOA, got: %s", result)
	}

	result = FormatCurrency(50000)
	if result != "500.00 AOA" {
		t.Errorf("expected 500.00 AOA, got: %s", result)
	}

	result = FormatCurrency(0)
	if result != "0.00 AOA" {
		t.Errorf("expected 0.00 AOA, got: %s", result)
	}
}

func TestFormatPhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"922123456", "+244 922 123 456"},
		{"+244922123456", "+244 922 123 456"},
		{"0922123456", "+244 922 123 456"},
		{"+244 912 123 456", "+244 912 123 456"},
	}

	for _, tt := range tests {
		result := FormatPhone(tt.input)
		if result != tt.expected {
			t.Errorf("FormatPhone(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"922123456", "244922123456", false},
		{"+244922123456", "244922123456", false},
		{"0922123456", "244922123456", false},
		{"+244 922 123 456", "244922123456", false},
		{"123", "", true},
		{"+244 222 123 456", "244222123456", false},
	}

	for _, tt := range tests {
		result, err := NormalizePhone(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("NormalizePhone(%s) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("NormalizePhone(%s) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("NormalizePhone(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		}
	}
}

func TestParseISODate(t *testing.T) {
	result := ParseISODate("2024-01-15T14:30:00Z")
	if result != "15/01/2024 14:30" {
		t.Errorf("expected 15/01/2024 14:30, got: %s", result)
	}
}
