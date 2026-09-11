package providers

import (
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/notifications"
	"go.uber.org/zap"
)

// DeliveryFactory creates notification delivery providers based on configuration.
type DeliveryFactory struct {
	logger *zap.Logger
}

// NewDeliveryFactory creates a new delivery factory.
func NewDeliveryFactory(logger *zap.Logger) *DeliveryFactory {
	return &DeliveryFactory{logger: logger}
}

// Create creates a delivery provider based on the provider name.
// Returns a NoopDeliveryProvider if no provider is configured.
func (f *DeliveryFactory) Create(
	provider string,
	// FCM options
	fcmAPIKey, fcmProjectID string,
	// APNs options
	apnsTeamID, apnsKeyID, apnsBundleID, apnsPrivateKey string,
	apnsProduction bool,
	// SMS fallback options
	otpDelivery auth.OTPDelivery,
	smsProvider string,
) notifications.DeliveryProvider {
	switch provider {
	case "fcm":
		f.logger.Info("creating FCM delivery provider")
		return NewFCMDelivery(fcmProjectID, fcmAPIKey)
	case "apns":
		f.logger.Info("creating APNs delivery provider")
		return NewAPNSDelivery(apnsTeamID, apnsKeyID, apnsBundleID, apnsPrivateKey, apnsProduction)
	case "sms":
		f.logger.Info("creating SMS fallback delivery provider")
		return NewSMSFallbackDelivery(otpDelivery, smsProvider)
	case "noop", "", "none":
		f.logger.Warn("using no-op delivery provider (development only)")
		return notifications.NoopDeliveryProvider{}
	default:
		f.logger.Warn("unknown delivery provider, using no-op", zap.String("provider", provider))
		return notifications.NoopDeliveryProvider{}
	}
}
