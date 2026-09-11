package providers

import (
	"context"

	"github.com/ridex/ridex-angola/internal/notifications"
)

// SMSSender is the narrow boundary a raw SMS transport must expose. The auth
// OTP providers (Termii, Africa's Talking, Twilio) implement it, letting the
// notification layer send pre-formatted text without any OTP framing.
type SMSSender interface {
	SendSMS(ctx context.Context, phone, message string) error
}

// SMSDelivery implements notifications.DeliveryProvider over a real SMS
// transport. It is an SMS-only channel: push attempts report not-configured
// so the delivery orchestrator knows to fall back, and SendSMS delegates to
// the transport verbatim — messages are never re-framed as OTP templates.
type SMSDelivery struct {
	sender   SMSSender
	provider string
}

// NewSMSDelivery wraps a raw SMS transport as a notification delivery
// provider. provider is the transport's name used for logging and debugging.
func NewSMSDelivery(sender SMSSender, provider string) *SMSDelivery {
	return &SMSDelivery{sender: sender, provider: provider}
}

// SendPush is not supported by an SMS-only provider.
func (d *SMSDelivery) SendPush(context.Context, string, notifications.PushNotification) error {
	return notifications.ErrDeliveryNotConfigured{Channel: "push"}
}

// SendSMS delivers the message verbatim through the wrapped transport.
func (d *SMSDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if d.sender == nil {
		return notifications.ErrDeliveryNotConfigured{Channel: "sms"}
	}
	return d.sender.SendSMS(ctx, phone, message)
}

// Name returns the provider name for logging and debugging.
func (d *SMSDelivery) Name() string {
	return "sms_" + d.provider
}
