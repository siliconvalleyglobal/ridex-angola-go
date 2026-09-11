package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/payment/providers"
)

// ErrPollerUnavailable means no provider status boundary is configured, so
// reconciliation cannot query live payment states.
var ErrPollerUnavailable = errors.New("payment status polling is not configured")

const (
	// reconcileDefaultLimit bounds one reconciliation pass when no explicit
	// limit is requested; reconcileMaxLimit is the hard ceiling.
	reconcileDefaultLimit = 25
	reconcileMaxLimit     = 100
)

// IntentStatus is the normalized outcome of polling a provider for a charge's
// live status.
type IntentStatus struct {
	ProviderChargeID string
	Status           string
}

// StatusPoller is the optional boundary for querying live payment status at a
// provider. Provider() names the single provider this poller can resolve, so
// the reconciliation pass skips charges recorded under other providers.
type StatusPoller interface {
	Provider() string
	GetPaymentStatus(ctx context.Context, providerChargeID string) (IntentStatus, error)
}

// ProviderStatusPoller adapts a providers.Provider implementation to the
// StatusPoller boundary.
type ProviderStatusPoller struct {
	Name   string
	Client providers.Provider
}

// Provider returns the normalized provider name this poller resolves.
func (p ProviderStatusPoller) Provider() string {
	return strings.ToLower(strings.TrimSpace(p.Name))
}

// GetPaymentStatus delegates to the provider adapter and normalizes the
// result, falling back to the queried identifier when the adapter does not
// echo the charge identifier.
func (p ProviderStatusPoller) GetPaymentStatus(ctx context.Context, providerChargeID string) (IntentStatus, error) {
	if p.Client == nil {
		return IntentStatus{}, ErrPollerUnavailable
	}
	payment, err := p.Client.GetPaymentStatus(ctx, providerChargeID)
	if err != nil {
		return IntentStatus{}, err
	}
	chargeID := payment.ProviderChargeID
	if chargeID == "" {
		chargeID = providerChargeID
	}
	return IntentStatus{ProviderChargeID: chargeID, Status: payment.Status}, nil
}

// ReconcileResult reports the outcome of one reconciliation pass.
type ReconcileResult struct {
	Checked int `json:"checked"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// canonicalProviderStatus maps a raw provider status string to the canonical
// ledger status. The mapping is deliberately conservative: unknown or
// ambiguous provider states map to an empty string and are skipped rather
// than guessed, because ApplyPaymentEventStatus guards the real transitions.
func canonicalProviderStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "completed", "paid", "succeeded", "success", "settled":
		return "completed"
	case "processing", "in_progress", "inprogress":
		return "processing"
	case "failed", "declined", "rejected":
		return "failed"
	case "refunded":
		return "refunded"
	default:
		return ""
	}
}

func canonicalEventType(status string) string {
	return "payment." + status
}

// reconcileEventID is deterministic so repeated passes over the same charge
// and status reuse one ledger event instead of duplicating rows.
func reconcileEventID(charge db.PaymentCharge, status string) string {
	return "recon:" + charge.ProviderChargeID + ":" + status
}

// reconcilePayload is deterministic (no timestamps) so a re-recorded event
// passes the ledger's idempotency payload comparison.
func reconcilePayload(status string) []byte {
	payload, _ := json.Marshal(map[string]string{
		"source": "reconciliation",
		"status": status,
	})
	return payload
}

// ReconcilePending polls the configured provider for charges stuck in
// pending/processing and applies canonical status transitions for webhooks
// that were missed. Per-charge failures never abort the pass; they are
// counted so the caller can alert on repeated failures. Charges recorded
// under providers other than the configured poller are skipped.
func (s *Service) ReconcilePending(ctx context.Context, limit int) (ReconcileResult, error) {
	if s.poller == nil {
		return ReconcileResult{}, ErrPollerUnavailable
	}
	if limit <= 0 {
		limit = reconcileDefaultLimit
	}
	if limit > reconcileMaxLimit {
		limit = reconcileMaxLimit
	}
	charges, err := s.store.GetPendingCharges(ctx, int32(limit))
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("list pending charges: %w", err)
	}

	result := ReconcileResult{}
	for _, charge := range charges {
		if charge.Provider != s.poller.Provider() || charge.ProviderChargeID == "" {
			result.Skipped++
			continue
		}
		result.Checked++
		status, err := s.poller.GetPaymentStatus(ctx, charge.ProviderChargeID)
		if err != nil {
			result.Failed++
			continue
		}
		canonical := canonicalProviderStatus(status.Status)
		if canonical == "" || canonical == "pending" || canonical == charge.Status {
			result.Skipped++
			continue
		}

		event, err := s.store.RecordPaymentEvent(ctx, db.RecordPaymentEventParams{
			ChargeID:        charge.ID,
			EventType:       canonicalEventType(canonical),
			Payload:         reconcilePayload(canonical),
			ProviderEventID: pgtype.Text{String: reconcileEventID(charge, canonical), Valid: true},
		})
		if err != nil {
			result.Failed++
			continue
		}
		if event.EventType != canonicalEventType(canonical) || !bytes.Equal(event.Payload, reconcilePayload(canonical)) {
			result.Failed++
			continue
		}
		if _, err := s.store.ApplyPaymentEventStatus(ctx, db.ApplyPaymentEventStatusParams{
			ID: charge.ID, Status: canonical,
		}); err != nil {
			result.Failed++
			continue
		}
		result.Updated++
	}
	return result, nil
}
