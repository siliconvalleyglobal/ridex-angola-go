package rides

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/db"
)

// CancellationReason represents why a ride was cancelled.
type CancellationReason string

const (
	CancellationReasonRiderNoShow     CancellationReason = "rider_no_show"
	CancellationReasonDriverNoShow    CancellationReason = "driver_no_show"
	CancellationReasonRiderRequested  CancellationReason = "rider_requested"
	CancellationReasonDriverRequested CancellationReason = "driver_requested"
	CancellationReasonNoDriverFound   CancellationReason = "no_driver_found"
	CancellationReasonSafetyConcern   CancellationReason = "safety_concern"
	CancellationReasonPaymentFailed   CancellationReason = "payment_failed"
	CancellationReasonOther           CancellationReason = "other"
)

// CancellationPolicy defines fees and rules for ride cancellations.
type CancellationPolicy struct {
	// FreeCancellationWindow is the time after ride creation during which cancellation is free.
	FreeCancellationWindow time.Duration

	// DriverAssignedFreeWindow is the time after driver assignment during which cancellation is free.
	DriverAssignedFreeWindow time.Duration

	// BaseFeeCents is the base cancellation fee in cents.
	BaseFeeCents int64

	// MaxFeeCents is the maximum cancellation fee in cents.
	MaxFeeCents int64

	// NoShowTimeout is how long to wait before marking a party as no-show.
	NoShowTimeout time.Duration

	// NoShowFeeCents is the fee charged for no-show.
	NoShowFeeCents int64
}

// DefaultCancellationPolicy returns the default cancellation policy for Angola.
func DefaultCancellationPolicy() CancellationPolicy {
	return CancellationPolicy{
		FreeCancellationWindow:   2 * time.Minute,
		DriverAssignedFreeWindow: 1 * time.Minute,
		BaseFeeCents:             50000,  // 500 AOA
		MaxFeeCents:              200000, // 2000 AOA
		NoShowTimeout:            5 * time.Minute,
		NoShowFeeCents:           100000, // 1000 AOA
	}
}

// CancellationResult describes the outcome of a cancellation request.
type CancellationResult struct {
	// Success indicates whether the cancellation was allowed.
	Success bool

	// FeeCents is the cancellation fee to be charged.
	FeeCents int64

	// Reason is the recorded cancellation reason.
	Reason CancellationReason

	// Message is a human-readable description.
	Message string
}

// ValidateCancellation checks if a cancellation is allowed and calculates any fees.
func ValidateCancellation(
	ride db.Ride,
	role string,
	reason CancellationReason,
	policy CancellationPolicy,
	now time.Time,
) (CancellationResult, error) {
	// Check if ride can be cancelled
	if ride.Status == "completed" {
		return CancellationResult{Success: false, Message: "ride is already completed"}, errors.New("ride already completed")
	}
	if ride.Status == "cancelled" {
		return CancellationResult{Success: false, Message: "ride is already cancelled"}, errors.New("ride already cancelled")
	}
	if ride.Status == "in_progress" && reason != CancellationReasonSafetyConcern {
		return CancellationResult{Success: false, Message: "cannot cancel ride in progress except for safety concerns"}, errors.New("cannot cancel ride in progress")
	}

	// Calculate cancellation fee
	feeCents := calculateCancellationFee(ride, role, reason, policy, now)

	// Drivers are never charged a cancellation fee: cancelling their own
	// accepted ride is an operational decision, not a chargeable event.
	if role == "driver" {
		feeCents = 0
	}
	return CancellationResult{
		Success:  true,
		FeeCents: feeCents,
		Reason:   reason,
		Message:  "cancellation allowed",
	}, nil
}

// calculateCancellationFee determines the cancellation fee based on timing and circumstances.
// Only riders can be charged; driver cancellations and no-driver no-shows are free.
func calculateCancellationFee(
	ride db.Ride,
	role string,
	reason CancellationReason,
	policy CancellationPolicy,
	now time.Time,
) int64 {
	// No fee for certain reasons
	switch reason {
	case CancellationReasonNoDriverFound, CancellationReasonSafetyConcern:
		return 0
	}

	// Drivers are never charged cancellation fees.
	if role == "driver" {
		return 0
	}

	// Check free cancellation window after ride creation
	if ride.CreatedAt.Valid {
		elapsed := now.Sub(ride.CreatedAt.Time)
		if elapsed < policy.FreeCancellationWindow {
			return 0
		}
	}

	// Check if driver is assigned
	if ride.DriverID.Valid {
		// Progressive fee based on time after driver assignment
		// Use UpdatedAt as proxy for when driver was assigned
		if ride.UpdatedAt.Valid {
			elapsed := now.Sub(ride.UpdatedAt.Time)
			if elapsed < policy.DriverAssignedFreeWindow {
				return 0
			}

			// Progressive fee based on time after free window
			overTime := elapsed - policy.DriverAssignedFreeWindow
			feeMultiplier := int64(overTime / (30 * time.Second))
			fee := policy.BaseFeeCents + (feeMultiplier * 10000) // Add 100 AOA per 30 seconds

			if fee > policy.MaxFeeCents {
				fee = policy.MaxFeeCents
			}
			return fee
		}
	}

	return policy.BaseFeeCents
}

// ProcessNoShow handles no-show detection and fee processing.
func ProcessNoShow(
	ride db.Ride,
	role string,
	policy CancellationPolicy,
	now time.Time,
) (CancellationResult, error) {
	if !ride.DriverID.Valid {
		return CancellationResult{Success: false, Message: "no driver assigned"}, errors.New("no driver assigned")
	}

	// Check if driver has been assigned long enough to be considered arrived
	// Use UpdatedAt as proxy for driver arrival time
	if ride.UpdatedAt.Valid {
		elapsed := now.Sub(ride.UpdatedAt.Time)
		if elapsed >= policy.NoShowTimeout {
			reason := CancellationReasonRiderNoShow
			feeCents := policy.NoShowFeeCents
			if role == "rider" {
				// A rider reporting a driver no-show is never charged; the
				// fee would be applicable to the driver, not the reporter.
				reason = CancellationReasonDriverNoShow
				feeCents = 0
			}

			return CancellationResult{
				Success:  true,
				FeeCents: feeCents,
				Reason:   reason,
				Message:  "no-show detected",
			}, nil
		}
	}

	return CancellationResult{Success: false, Message: "no-show timeout not reached"}, nil
}

// CancellationRecord represents a cancellation event for audit purposes.
type CancellationRecord struct {
	RideID            uuid.UUID
	CancelledBy       uuid.UUID
	CancelledRole     string
	Reason            CancellationReason
	FeeCents          int64
	RefundAmountCents int64
	CreatedAt         time.Time
}
