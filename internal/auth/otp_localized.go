package auth

import (
	"context"
	"fmt"

	"github.com/ridex/ridex-angola/internal/i18n/angola"
)

// LocalizedOTPDelivery wraps an OTPDelivery with Portuguese (Angola) localization.
// It translates OTP purpose keys into localized SMS messages before delivery.
type LocalizedOTPDelivery struct {
	delivery   OTPDelivery
	translator *angola.Translator
}

// NewLocalizedOTPDelivery creates a new localized OTP delivery wrapper.
func NewLocalizedOTPDelivery(delivery OTPDelivery) *LocalizedOTPDelivery {
	return &LocalizedOTPDelivery{
		delivery:   delivery,
		translator: angola.NewTranslator(),
	}
}

// DeliverOTP sends a localized OTP message via the wrapped delivery provider.
// The formatted message is transmitted verbatim when the wrapped provider
// exposes a raw SMS transport (SendSMS); the legacy DeliverOTP path remains
// for simple mocks so the message is never re-framed as an OTP template.
func (d *LocalizedOTPDelivery) DeliverOTP(ctx context.Context, phone, purpose, code string) error {
	return d.SendSMS(ctx, phone, d.formatMessage(purpose, code))
}

// SendSMS delivers a pre-formatted SMS message through the wrapped provider
// without any OTP framing.
func (d *LocalizedOTPDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if sender, ok := d.delivery.(SMSSender); ok {
		return sender.SendSMS(ctx, phone, message)
	}
	// Fallback for providers without a raw SMS primitive: DeliverOTP with the
	// "sms" purpose transmits the pre-formatted message.
	return d.delivery.DeliverOTP(ctx, phone, "sms", message)
}

// formatMessage creates a localized OTP message based on purpose.
func (d *LocalizedOTPDelivery) formatMessage(purpose, code string) string {
	switch purpose {
	case "login":
		return fmt.Sprintf(d.translator.T(angola.MsgOTPLogin), code)
	case "registration":
		return fmt.Sprintf(d.translator.T(angola.MsgOTPRegister), code)
	case "password_reset":
		return fmt.Sprintf(d.translator.T(angola.MsgOTPPasswordReset), code)
	case "phone_change":
		return fmt.Sprintf(d.translator.T(angola.MsgOTPPhoneChange), code)
	default:
		return fmt.Sprintf(d.translator.T(angola.MsgOTPVerificationCode), code)
	}
}
