// Package providers implements payment provider adapters for RideX Angola.
// Each adapter translates between the provider-neutral payment ledger and
// a specific payment provider's API.
package providers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Provider is the interface that all payment providers must implement.
type Provider interface {
	// CreatePaymentIntent creates a payment intent with the provider.
	CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error)

	// GetPaymentStatus queries the status of a payment intent.
	GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error)

	// RefundPayment refunds a payment.
	RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error

	// VerifyWebhookSignature verifies webhook signatures.
	VerifyWebhookSignature(payload []byte, signature string) error

	// ParseWebhookEvent parses and validates a webhook event.
	ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error)
}

// Payment is the common payment structure returned by providers.
type Payment struct {
	ID               string
	ProviderChargeID string
	Status           string
	Amount           int64
	Currency         string
	CreatedAt        string
	CompletedAt      string
}

// WebhookEvent is the common webhook event structure.
type WebhookEvent struct {
	ID               string
	Type             string
	ProviderChargeID string
	Data             interface{}
	CreatedAt        string
	Signature        string
}

// Registry holds all configured payment providers.
type Registry struct {
	providers map[string]Provider
	logger    *zap.Logger
}

// NewRegistry creates a new provider registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		logger:    logger,
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(name string, provider Provider) {
	r.providers[name] = provider
	r.logger.Info("registered payment provider", zap.String("provider", name))
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("payment provider %s not found", name)
	}
	return provider, nil
}

// List returns all registered providers.
func (r *Registry) List() []string {
	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
