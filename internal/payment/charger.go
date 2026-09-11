package payment

import (
	"context"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/payment/providers"
)

// IntentResult is the provider-neutral outcome of creating a remote payment
// intent.
type IntentResult struct {
	ProviderChargeID string
	Status           string
}

// Charger is the optional boundary for creating real payment intents at a
// configured provider. A nil charger puts the ledger in local-only mode where
// intents are recorded with an opaque identifier and no provider call is
// made.
type Charger interface {
	CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*IntentResult, error)
}

// ProviderCharger adapts a providers.Provider implementation to the Charger
// boundary so the payment ledger stays independent of any provider package.
type ProviderCharger struct {
	Provider providers.Provider
}

// CreatePaymentIntent delegates to the provider and normalizes the result.
func (c ProviderCharger) CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*IntentResult, error) {
	p, err := c.Provider.CreatePaymentIntent(ctx, rideID, amountCents, currency)
	if err != nil {
		return nil, err
	}
	return &IntentResult{ProviderChargeID: p.ProviderChargeID, Status: p.Status}, nil
}
