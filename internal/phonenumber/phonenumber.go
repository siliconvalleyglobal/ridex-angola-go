package phonenumber

import (
	"fmt"
	"regexp"
	"strings"
)

// Angola country code
const AngolaCountryCode = "244"

// Valid Angolan mobile prefixes (9xx where xx is 12-19, 21-29)
var validMobilePrefixes = map[string]bool{
	"91": true, "92": true, "93": true, "94": true, "95": true,
}

// Valid Angolan landline prefixes (2xx where xx is 22, 24, 25, 28)
var validLandlinePrefixes = map[string]bool{
	"22": true, "24": true, "25": true, "28": true,
}

// PhoneNumber represents a normalized Angolan phone number
type PhoneNumber struct {
	// Raw is the original input
	Raw string

	// Normalized is the E.164 format: +244XXXXXXXX
	Normalized string

	// Type indicates if it's mobile or landline
	Type PhoneType

	// HasValidFormat indicates if the number matches Angolan patterns
	HasValidFormat bool
}

// PhoneType indicates the type of phone number
type PhoneType string

const (
	PhoneTypeMobile   PhoneType = "mobile"
	PhoneTypeLandline PhoneType = "landline"
	PhoneTypeUnknown  PhoneType = "unknown"
)

// NormalizePhoneNumber normalizes an Angolan phone number to E.164 format
func NormalizePhoneNumber(input string) PhoneNumber {
	result := PhoneNumber{
		Raw:            input,
		Type:           PhoneTypeUnknown,
		HasValidFormat: false,
	}

	clean := strings.TrimSpace(input)
	startsWithPlus := strings.HasPrefix(clean, "+")

	reg := regexp.MustCompile(`[^\d]`)
	digitsOnly := reg.ReplaceAllString(clean, "")

	// Handle prefix normalization
	var normalized string
	var localDigits string

	switch {
	case startsWithPlus && strings.HasPrefix(digitsOnly, AngolaCountryCode):
		// +244XXXXXXXX (12 digits after +)
		normalized = "+" + digitsOnly
		localDigits = digitsOnly[len(AngolaCountryCode):]

	case !startsWithPlus && strings.HasPrefix(digitsOnly, AngolaCountryCode) && len(digitsOnly) == 12:
		// 244XXXXXXXX (12 digits without +)
		normalized = "+" + digitsOnly
		localDigits = digitsOnly[len(AngolaCountryCode):]

	case len(digitsOnly) == 10 && digitsOnly[0] == '0':
		// 0XXXXXXXXX (10 digits, domestic format with leading 0)
		normalized = "+" + AngolaCountryCode + digitsOnly[1:]
		localDigits = digitsOnly[1:]

	case len(digitsOnly) == 9:
		// XXXXXXXXX (9 digits, mobile-only convention in Angola)
		normalized = "+" + AngolaCountryCode + digitsOnly
		localDigits = digitsOnly

	default:
		return result
	}

	// Validate
	if len(normalized) != 13 || !strings.HasPrefix(normalized, "+") {
		return result
	}

	if len(localDigits) != 9 {
		return result
	}

	// Check if the prefix is valid
	prefix := localDigits[0:2]
	if !isValidPrefix(prefix, localDigits[0]) {
		return result
	}

	result.Normalized = normalized
	result.HasValidFormat = true

	// Determine type based on prefix
	if localDigits[0] == '9' {
		result.Type = PhoneTypeMobile
	} else if localDigits[0] == '2' {
		result.Type = PhoneTypeLandline
	}

	return result
}

// isValidPrefix checks if the 2-digit prefix is valid for the given first digit
func isValidPrefix(prefix string, firstDigit byte) bool {
	if firstDigit == '9' {
		return validMobilePrefixes[prefix]
	} else if firstDigit == '2' {
		return validLandlinePrefixes[prefix]
	}
	return false
}

// FormatForDisplay formats the phone number for display in Angola
func FormatForDisplay(phone PhoneNumber) string {
	if !phone.HasValidFormat || phone.Normalized == "" {
		return phone.Raw
	}

	localNumber := phone.Normalized[4:] // Remove "+244"

	if len(localNumber) != 9 {
		return phone.Normalized
	}

	return fmt.Sprintf("%s %s %s", localNumber[0:3], localNumber[3:6], localNumber[6:9])
}

// FormatForE164 returns the E.164 format (+244912123456)
func FormatForE164(phone PhoneNumber) string {
	return phone.Normalized
}

// FormatForDomestic returns the domestic format (0912123456)
func FormatForDomestic(phone PhoneNumber) string {
	if !phone.HasValidFormat || phone.Normalized == "" {
		return phone.Raw
	}

	localNumber := phone.Normalized[4:] // Remove "+244"

	if len(localNumber) != 9 {
		return phone.Normalized
	}

	return "0" + localNumber
}

// IsValidAngolanMobile checks if the number is a valid Angolan mobile number
func IsValidAngolanMobile(input string) bool {
	phone := NormalizePhoneNumber(input)
	return phone.HasValidFormat && phone.Type == PhoneTypeMobile
}

// IsValidAngolanLandline checks if the number is a valid Angolan landline
func IsValidAngolanLandline(input string) bool {
	phone := NormalizePhoneNumber(input)
	return phone.HasValidFormat && phone.Type == PhoneTypeLandline
}

// ExtractDigits extracts only digits from a phone number string
func ExtractDigits(input string) string {
	reg := regexp.MustCompile(`[^\d]`)
	return reg.ReplaceAllString(strings.TrimSpace(input), "")
}
