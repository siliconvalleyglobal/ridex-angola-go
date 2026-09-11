package providers

import (
	"context"

	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/notifications"
)

// SMSFallbackDelivery implements DeliveryProvider using SMS as a fallback channel.
// It wraps the existing OTP delivery providers (Termii, Africa's Talking, Twilio)
// to send notification messages via SMS when push notifications are unavailable.
type SMSFallbackDelivery struct {
	otpDelivery auth.OTPDelivery
	provider    string
}

// NewSMSFallbackDelivery creates a new SMS fallback delivery provider.
func NewSMSFallbackDelivery(otpDelivery auth.OTPDelivery, provider string) *SMSFallbackDelivery {
	return &SMSFallbackDelivery{
		otpDelivery: otpDelivery,
		provider:    provider,
	}
}

// SendPush is not supported by SMS fallback provider.
func (d *SMSFallbackDelivery) SendPush(context.Context, string, notifications.PushNotification) error {
	return notifications.ErrDeliveryNotConfigured{Channel: "push"}
}

// SendSMS sends an SMS message via the configured OTP provider.
func (d *SMSFallbackDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if d.otpDelivery == nil {
		return notifications.ErrDeliveryNotConfigured{Channel: "sms"}
	}
	// Use "sms" as purpose since we're sending a pre-formatted message
	return d.otpDelivery.DeliverOTP(ctx, phone, "sms", message)
}

// Name returns the provider name.
func (d *SMSFallbackDelivery) Name() string {
	return "sms_" + d.provider
}
