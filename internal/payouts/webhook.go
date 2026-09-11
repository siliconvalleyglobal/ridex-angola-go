package payouts

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ridex/ridex-angola/internal/db"
)

// Webhook errors surfaced to the HTTP layer.
var (
	ErrWebhookNotConfigured = errors.New("payout webhook secret is not configured")
	ErrInvalidWebhook       = errors.New("invalid payout webhook signature")
	ErrInvalidCallback      = errors.New("invalid payout callback status")
)

// maxWebhookSecretBody caps callback bodies so a hostile caller cannot
// exhaust memory; mirrors the payment webhook limit.
const maxWebhookSecretBody = 1 << 20

// VerifyWebhookSignature checks a hex-encoded HMAC-SHA256 signature over the
// raw callback body with the configured payout webhook secret. The signature
// header may carry an optional "sha256=" prefix, mirroring the payment
// verifiers. An unset secret fails closed: callbacks are never accepted on
// trust.
func VerifyWebhookSignature(payload []byte, signature, secret string) error {
	if strings.TrimSpace(secret) == "" {
		return ErrWebhookNotConfigured
	}
	signature = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(signature), "sha256="))
	decoded, err := hex.DecodeString(signature)
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("%w: signature must be hex-encoded sha256", ErrInvalidWebhook)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	if !hmac.Equal(decoded, mac.Sum(nil)) {
		return ErrInvalidWebhook
	}
	return nil
}

// PayoutCallback is the parsed executor callback for one payout.
type PayoutCallback struct {
	PayoutID uuid.UUID
	Status   string // StatusCompleted, StatusFailed or StatusProcessing
	Message  string
}

// RecordPayoutCallback applies an executor's push notification for one payout.
// Completion settles through the guarded CompletePayout transition; failure
// reverses the withdrawal through FailPayout. A processing callback is
// accepted as a no-op. Every transition runs through the ledger's SQL guards,
// so duplicated or late callbacks are idempotent, and a callback for a payout
// already settled reports ErrPayoutTransition rather than double-applying.
func (l *Ledger) RecordPayoutCallback(ctx context.Context, callback PayoutCallback) (db.PayoutRequest, error) {
	switch callback.Status {
	case StatusCompleted:
		return l.CompletePayout(ctx, callback.PayoutID)
	case StatusFailed:
		reason := "provider callback"
		if callback.Message != "" {
			reason = "provider callback: " + callback.Message
		}
		return l.FailPayout(ctx, callback.PayoutID, reason)
	case StatusProcessing:
		request, err := l.store.GetPayoutRequestByID(ctx, callback.PayoutID)
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PayoutRequest{}, ErrPayoutNotFound
		}
		if err != nil {
			return db.PayoutRequest{}, fmt.Errorf("load payout request: %w", err)
		}
		return request, nil
	default:
		return db.PayoutRequest{}, fmt.Errorf("%w: %q", ErrInvalidCallback, callback.Status)
	}
}
