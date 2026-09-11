// Package auth implements OTP factory for creating delivery providers.
package auth

import (
	"go.uber.org/zap"
)

// OTPDeliveryFactory creates OTPDelivery instances based on configuration.
type OTPDeliveryFactory struct {
	logger *zap.Logger
}

// NewOTPDeliveryFactory creates a new OTP delivery factory.
func NewOTPDeliveryFactory(logger *zap.Logger) *OTPDeliveryFactory {
	return &OTPDeliveryFactory{logger: logger}
}

// Create creates an OTPDelivery instance based on the provider name.
func (f *OTPDeliveryFactory) Create(
	provider string,
	termiiAPIKey, termiiSenderID string,
	atAPIClientKey, atUsername string,
	twilioAccountSID, twilioAuthToken, twilioFrom string,
) OTPDelivery {
	switch provider {
	case "termii":
		f.logger.Info("creating Termii OTP delivery")
		return NewTermiiDelivery(termiiAPIKey, termiiSenderID)
	case "africastalking":
		f.logger.Info("creating Africa's Talking OTP delivery")
		return NewAfricaTalkingDelivery(atAPIClientKey, atUsername)
	case "twilio":
		f.logger.Info("creating Twilio OTP delivery")
		return NewTwilioDelivery(twilioAccountSID, twilioAuthToken, twilioFrom)
	case "noop", "", "none":
		f.logger.Warn("using no-op OTP delivery (development only)")
		return NoopOTPDelivery{}
	default:
		f.logger.Warn("unknown OTP provider, using no-op", zap.String("provider", provider))
		return NoopOTPDelivery{}
	}
}
