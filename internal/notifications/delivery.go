package notifications

import (
	"context"
)

// DeliveryProvider is the boundary for external notification delivery.
// Implementations may send push notifications via FCM/APNs or SMS fallback.
// No provider calls are attempted until a provider is selected and configured.
type DeliveryProvider interface {
	// SendPush sends a push notification to the specified device token.
	// Returns ErrDeliveryNotConfigured if no provider is configured.
	SendPush(ctx context.Context, deviceToken string, notification PushNotification) error

	// SendSMS sends an SMS message to the specified phone number.
	// Returns ErrDeliveryNotConfigured if no provider is configured.
	SendSMS(ctx context.Context, phone, message string) error

	// Name returns the provider name for logging and debugging.
	Name() string
}

// PushNotification represents a push notification payload.
type PushNotification struct {
	Title    string
	Body     string
	Data     map[string]string
	Priority string // "high" or "normal"
}

// ErrDeliveryNotConfigured indicates no external delivery provider is configured.
// It is intentionally distinct from a provider failure so that the application
// can persist notifications without falsely claiming they were delivered.
type ErrDeliveryNotConfigured struct {
	Channel string // "push" or "sms"
}

func (e ErrDeliveryNotConfigured) Error() string {
	return e.Channel + " delivery is not configured"
}

// NoopDeliveryProvider is the default delivery provider when no external
// provider is configured. It records no delivery and returns an explicit
// unavailable error. It never claims that a notification was sent.
type NoopDeliveryProvider struct{}

func (NoopDeliveryProvider) SendPush(context.Context, string, PushNotification) error {
	return ErrDeliveryNotConfigured{Channel: "push"}
}

func (NoopDeliveryProvider) SendSMS(context.Context, string, string) error {
	return ErrDeliveryNotConfigured{Channel: "sms"}
}

func (NoopDeliveryProvider) Name() string {
	return "noop"
}

// DeliveryService orchestrates external notification delivery with fallback.
// It attempts push notification first, then SMS if push fails or is unavailable.
type DeliveryService struct {
	provider DeliveryProvider
}

// NewDeliveryService creates a new delivery service with the specified provider.
func NewDeliveryService(provider DeliveryProvider) *DeliveryService {
	if provider == nil {
		provider = NoopDeliveryProvider{}
	}
	return &DeliveryService{provider: provider}
}

// Notify sends a notification via the configured provider.
// It attempts push first, then falls back to SMS if configured.
func (s *DeliveryService) Notify(ctx context.Context, input DeliveryInput) (*DeliveryResult, error) {
	result := &DeliveryResult{}

	// Attempt push notification
	if input.DeviceToken != "" {
		pushErr := s.provider.SendPush(ctx, input.DeviceToken, PushNotification{
			Title:    input.Title,
			Body:     input.Body,
			Data:     input.Data,
			Priority: input.Priority,
		})
		if pushErr == nil {
			result.PushDelivered = true
			result.Provider = s.provider.Name()
			return result, nil
		}
		// If push failed for reasons other than not configured, don't try SMS
		if _, ok := pushErr.(ErrDeliveryNotConfigured); !ok {
			return result, pushErr
		}
	}

	// Fall back to SMS if phone is provided
	if input.Phone != "" {
		smsErr := s.provider.SendSMS(ctx, input.Phone, input.Title+". "+input.Body)
		if smsErr == nil {
			result.SMSDelivered = true
			result.Provider = s.provider.Name()
			return result, nil
		}
		return result, smsErr
	}

	return result, ErrDeliveryNotConfigured{Channel: "push"}
}

// DeliveryInput describes a notification to be delivered externally.
type DeliveryInput struct {
	UserID      string
	DeviceToken string
	Phone       string
	Title       string
	Body        string
	Data        map[string]string
	Priority    string
}

// DeliveryResult describes the outcome of a delivery attempt.
type DeliveryResult struct {
	PushDelivered bool
	SMSDelivered  bool
	Provider      string
}
