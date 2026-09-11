package payouts

import "context"

// AuditLogger emits payout lifecycle events. Keeping it behind an interface
// lets the core ledger stay provider-independent: real delivery (database,
// external audit queue, or SaaS) is wired at the edge.
type AuditLogger interface {
	LogPayoutEvent(ctx context.Context, event PayoutEvent) error
}

// PayoutEvent is a single, immutable lifecycle event for a payout request.
type PayoutEvent struct {
	PayoutID   string
	DriverID   string
	ActorID    string
	AmountCents int64
	Status      string
	Method      string
	Reason      string
}

// noopAuditLogger is the in-process implementation used when no external audit
// system has been selected. It exists so the ledger can always call the logger
// without branching on "is audit configured?".
type noopAuditLogger struct{}

func (noopAuditLogger) LogPayoutEvent(_ context.Context, _ PayoutEvent) error { return nil }

// NewNoopAuditLogger returns a logger that discards events. Use it during early
// development and when no audit sink has been wired.
func NewNoopAuditLogger() AuditLogger { return noopAuditLogger{} }

// RecordPayoutEvent emits a payout lifecycle event through the ledger's logger.
// It is best-effort: a failure to log never blocks the underlying payout
// mutation, because the money movement is already transactional.
func RecordPayoutEvent(log AuditLogger, ctx context.Context, e PayoutEvent) {
	if log == nil {
		return
	}
	_ = log.LogPayoutEvent(ctx, e)
}
