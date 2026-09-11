package phonenumber

import (
	"testing"
)

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedNorm string
		expectedType PhoneType
		expectValid  bool
	}{
		{
			name:         "International format with spaces",
			input:        "+244 912 123 456",
			expectedNorm: "+244912123456",
			expectedType: PhoneTypeMobile,
			expectValid:  true,
		},
		{
			name:         "International format without spaces",
			input:        "+244912123456",
			expectedNorm: "+244912123456",
			expectedType: PhoneTypeMobile,
			expectValid:  true,
		},
		{
			name:         "Domestic format with leading 0",
			input:        "0912123456",
			expectedNorm: "+244912123456",
			expectedType: PhoneTypeMobile,
			expectValid:  true,
		},
		{
			name:         "Country code without plus",
			input:        "244912123456",
			expectedNorm: "+244912123456",
			expectedType: PhoneTypeMobile,
			expectValid:  true,
		},
		{
			name:         "With hyphens",
			input:        "+244-912-123-456",
			expectedNorm: "+244912123456",
			expectedType: PhoneTypeMobile,
			expectValid:  true,
		},
		{
			name:         "Landline number",
			input:        "+244 222 123 456",
			expectedNorm: "+244222123456",
			expectedType: PhoneTypeLandline,
			expectValid:  true,
		},
		{
			name:        "Invalid mobile prefix (999)",
			input:       "+244 999 123 456",
			expectValid: false,
		},
		{
			name:        "Too short",
			input:       "+244 912 123",
			expectValid: false,
		},
		{
			name:        "Too long",
			input:       "+244 912 123 456 789",
			expectValid: false,
		},
		{
			name:        "Wrong country code",
			input:       "+1 234 567 8901",
			expectValid: false,
		},
		{
			name:        "Empty string",
			input:       "",
			expectValid: false,
		},
		{
			name:        "Nil-like input",
			input:       "---",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePhoneNumber(tt.input)

			if tt.expectValid {
				if !result.HasValidFormat {
					t.Errorf("expected valid format, got invalid")
				}
				if result.Normalized != tt.expectedNorm {
					t.Errorf("expected normalized %s, got %s", tt.expectedNorm, result.Normalized)
				}
				if result.Type != tt.expectedType {
					t.Errorf("expected type %s, got %s", tt.expectedType, result.Type)
				}
			} else {
				if result.HasValidFormat {
					t.Errorf("expected invalid format, got valid")
				}
			}
		})
	}
}

func TestFormatForDisplay(t *testing.T) {
	phone := PhoneNumber{
		Raw:            "+244 912 123 456",
		Normalized:     "+244912123456",
		Type:           PhoneTypeMobile,
		HasValidFormat: true,
	}

	result := FormatForDisplay(phone)

	if result != "912 123 456" {
		t.Errorf("expected '912 123 456', got '%s'", result)
	}
}

func TestFormatForE164(t *testing.T) {
	phone := PhoneNumber{
		Raw:            "+244 912 123 456",
		Normalized:     "+244912123456",
		Type:           PhoneTypeMobile,
		HasValidFormat: true,
	}

	result := FormatForE164(phone)

	if result != "+244912123456" {
		t.Errorf("expected '+244912123456', got '%s'", result)
	}
}

func TestFormatForDomestic(t *testing.T) {
	phone := PhoneNumber{
		Raw:            "+244 912 123 456",
		Normalized:     "+244912123456",
		Type:           PhoneTypeMobile,
		HasValidFormat: true,
	}

	result := FormatForDomestic(phone)

	if result != "0912123456" {
		t.Errorf("expected '0912123456', got '%s'", result)
	}
}

func TestIsValidAngolanMobile(t *testing.T) {
	tests := []struct {
		input       string
		expectValid bool
	}{
		{"+244 912 123 456", true},
		{"0912123456", true},
		{"+244922123456", true},
		{"+244 999 123 456", false},
		{"+1 234 567 8901", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidAngolanMobile(tt.input)
			if result != tt.expectValid {
				t.Errorf("expected %v, got %v", tt.expectValid, result)
			}
		})
	}
}

func TestIsValidAngolanLandline(t *testing.T) {
	tests := []struct {
		input       string
		expectValid bool
	}{
		{"+244 222 123 456", true},
		{"+244 244 123 456", true},
		{"+244 912 123 456", false}, // It's mobile
		{"+1 234 567 8901", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidAngolanLandline(tt.input)
			if result != tt.expectValid {
				t.Errorf("expected %v, got %v", tt.expectValid, result)
			}
		})
	}
}

func TestExtractDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+244 912 123 456", "244912123456"},
		{"244-912-123-456", "244912123456"},
		{"(244) 912-123-456", "244912123456"},
		{"912 123 456", "912123456"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractDigits(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
