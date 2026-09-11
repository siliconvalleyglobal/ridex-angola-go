package payouts

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Executor submits and polls an external payout (bank transfer, Multicaixa, or
// mobile money). It is the provider-neutral boundary between the payout ledger
// and a money-movement provider, mirroring payment's Charger/Refunder. A
// submission never reports final success/failure for the transfer itself — the
// caller polls Status afterwards and only then settles or reverses the ledger.
type Executor interface {
	// Submit initiates an external payout of amountCents to the driver through
	// the given method. reference is the payout_requests row id and the
	// provider must honor it as the submission's idempotency key: resubmitting
	// with the same reference must never pay twice. Errors wrapping
	// ErrSubmitUncertain leave the outcome unknown (the provider may have
	// processed the payout); only definite rejections may reverse the wallet.
	Submit(method string, amountCents int64, driverID uuid.UUID, reference string) (Submission, error)
	// Status returns the current external state for a previously submitted
	// payout. Polling is only meaningful for references the executor issued.
	Status(reference string) (StatusInfo, error)
}

// ErrSubmitUncertain marks a Submit failure where the provider may already
// have processed the payout (lost response, 5xx, unparseable reply). The
// ledger holds the payout in flight and retries with the same reference —
// which providers must treat as the idempotency key — instead of reversing,
// because a reversal while the transfer is actually in flight would pay twice.
var ErrSubmitUncertain = errors.New("payout submission outcome is uncertain")

// Submission is the immediate result of Submit.
type Submission struct {
	Reference string
	Status    string // StatusProcessing, StatusCompleted or StatusFailed
	Message   string
}

// StatusInfo is the result of a Status poll.
type StatusInfo struct {
	Reference string
	Status    string // StatusProcessing, StatusCompleted or StatusFailed
	Message   string
}

// Executor statuses mirror the payout_requests statuses for the transitions
// the worker drives. Unknown provider states are skipped, never guessed.
const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// ManualExecutor is the deterministic, in-process fallback used before a real
// banking/Multicaixa provider is configured. Submissions are recorded in
// memory with a stable reference and reported as processing; Status returns
// whatever the last recorded state was. Because the state is process-local, a
// fresh worker never fabricates completions for payouts it did not submit —
// with no real provider wired, payouts stay in flight and admins complete or
// fail them through the existing admin endpoints.
type ManualExecutor struct {
	states map[string]string
}

// NewManualExecutor creates an executor that records submissions in memory.
func NewManualExecutor() *ManualExecutor {
	return &ManualExecutor{states: make(map[string]string)}
}

// Submit validates the inputs and records a processing submission.
func (e *ManualExecutor) Submit(method string, amountCents int64, driverID uuid.UUID, reference string) (Submission, error) {
	if strings.TrimSpace(method) == "" {
		return Submission{}, errors.New("payout method is required")
	}
	if amountCents <= 0 {
		return Submission{}, errors.New("payout amount must be positive")
	}
	if driverID == uuid.Nil {
		return Submission{}, errors.New("driver is required")
	}
	providerReference := "payout-" + uuid.New().String()
	e.states[providerReference] = StatusProcessing
	return Submission{Reference: providerReference, Status: StatusProcessing, Message: "submitted to manual executor"}, nil
}

// Status returns the recorded state for a reference the executor issued.
func (e *ManualExecutor) Status(reference string) (StatusInfo, error) {
	state, ok := e.states[reference]
	if !ok {
		return StatusInfo{}, errors.New("unknown payout reference: " + reference)
	}
	return StatusInfo{Reference: reference, Status: state, Message: "manual executor status"}, nil
}

// Record advances a manual submission's state, making the in-process loop
// testable (a real provider would drive this externally).
func (e *ManualExecutor) Record(reference, status string) {
	e.states[reference] = status
}
