package auth

import (
	"context"
	"errors"
)

// ErrOTPDeliveryNotConfigured means the application has no selected provider.
// It is intentionally distinct from a provider failure so a challenge can be
// created without falsely claiming that an SMS was sent.
var ErrOTPDeliveryNotConfigured = errors.New("OTP delivery is not configured")

// OTPDelivery is the boundary for a future documented SMS or messaging
// provider. Implementations must not expose the plaintext code in logs.
type OTPDelivery interface {
	DeliverOTP(ctx context.Context, phone, purpose, code string) error
}

// SMSSender is implemented by delivery providers that can also transmit
// pre-formatted SMS text without OTP framing. Localized OTP messages and the
// notification SMS adapters use it so their text is delivered verbatim.
type SMSSender interface {
	SendSMS(ctx context.Context, phone, message string) error
}

// NoopOTPDelivery records no delivery and returns an explicit unavailable
// error. It never claims that an SMS was sent.
type NoopOTPDelivery struct{}

func (NoopOTPDelivery) DeliverOTP(context.Context, string, string, string) error {
	return ErrOTPDeliveryNotConfigured
}
