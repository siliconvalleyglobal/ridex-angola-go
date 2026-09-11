package rides

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

func TestValidateCancellation_RideCompleted(t *testing.T) {
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "completed",
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, time.Now())
	if err == nil {
		t.Error("ValidateCancellation() should fail for completed ride")
	}
	if result.Success {
		t.Error("ValidateCancellation() should not succeed for completed ride")
	}
}

func TestValidateCancellation_FreeWindow(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "requested",
		CreatedAt: pgtype.Timestamptz{
			Time:  now.Add(-30 * time.Second), // 30 seconds ago
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed within free window")
	}
	if result.FeeCents != 0 {
		t.Errorf("ValidateCancellation() fee = %d, want 0", result.FeeCents)
	}
}

func TestValidateCancellation_AfterFreeWindow(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "requested",
		CreatedAt: pgtype.Timestamptz{
			Time:  now.Add(-5 * time.Minute), // 5 minutes ago
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed")
	}
	if result.FeeCents != policy.BaseFeeCents {
		t.Errorf("ValidateCancellation() fee = %d, want %d", result.FeeCents, policy.BaseFeeCents)
	}
}

func TestValidateCancellation_NoDriverFound(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "matching",
		CreatedAt: pgtype.Timestamptz{
			Time:  now.Add(-10 * time.Minute),
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonNoDriverFound, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed")
	}
	if result.FeeCents != 0 {
		t.Errorf("ValidateCancellation() fee = %d, want 0 for no driver found", result.FeeCents)
	}
}

func TestValidateCancellation_SafetyConcern(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "in_progress",
		CreatedAt: pgtype.Timestamptz{
			Time:  now.Add(-10 * time.Minute),
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonSafetyConcern, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed for safety concern")
	}
	if result.FeeCents != 0 {
		t.Errorf("ValidateCancellation() fee = %d, want 0 for safety concern", result.FeeCents)
	}
}

func TestValidateCancellation_InProgressNotSafety(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "in_progress",
	}
	policy := DefaultCancellationPolicy()

	_, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, now)
	if err == nil {
		t.Error("ValidateCancellation() should fail for non-safety cancellation of in-progress ride")
	}
}

func TestProcessNoShow_DriverArrived(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "driver_assigned",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:  now.Add(-6 * time.Minute), // 6 minutes ago, past 5 min timeout
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ProcessNoShow(ride, "driver", policy, now)
	if err != nil {
		t.Errorf("ProcessNoShow() error = %v", err)
	}
	if !result.Success {
		t.Error("ProcessNoShow() should succeed after timeout")
	}
	if result.FeeCents != policy.NoShowFeeCents {
		t.Errorf("ProcessNoShow() fee = %d, want %d", result.FeeCents, policy.NoShowFeeCents)
	}
}

func TestProcessNoShow_BeforeTimeout(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "driver_assigned",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:  now.Add(-2 * time.Minute), // 2 minutes ago, before 5 min timeout
			Valid: true,
		},
	}
	policy := DefaultCancellationPolicy()

	result, err := ProcessNoShow(ride, "rider", policy, now)
	if err != nil {
		t.Errorf("ProcessNoShow() error = %v", err)
	}
	if result.Success {
		t.Error("ProcessNoShow() should not succeed before timeout")
	}
}

func TestDefaultCancellationPolicy(t *testing.T) {
	policy := DefaultCancellationPolicy()

	if policy.FreeCancellationWindow != 2*time.Minute {
		t.Errorf("FreeCancellationWindow = %v, want 2m", policy.FreeCancellationWindow)
	}
	if policy.BaseFeeCents != 50000 {
		t.Errorf("BaseFeeCents = %d, want 50000", policy.BaseFeeCents)
	}
	if policy.NoShowFeeCents != 100000 {
		t.Errorf("NoShowFeeCents = %d, want 100000", policy.NoShowFeeCents)
	}
}

func TestValidateCancellation_DriverRoleIsFree(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "matched",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		CreatedAt: pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "driver", CancellationReasonDriverRequested, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed for driver")
	}
	if result.FeeCents != 0 {
		t.Errorf("driver cancellation fee = %d, want 0", result.FeeCents)
	}
}

func TestValidateCancellation_ProgressiveFeeCapsAtMax(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "driver_arriving",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		// Long after both windows: fee should hit the MaxFeeCents cap.
		CreatedAt: pgtype.Timestamptz{Time: now.Add(-60 * time.Minute), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now.Add(-55 * time.Minute), Valid: true},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if !result.Success {
		t.Error("ValidateCancellation() should succeed")
	}
	if result.FeeCents != policy.MaxFeeCents {
		t.Errorf("fee = %d, want capped at MaxFeeCents %d", result.FeeCents, policy.MaxFeeCents)
	}
}

func TestValidateCancellation_DriverAssignedFreeWindow(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "matched",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		// Created long ago but driver assigned 30s ago: inside the
		// 1-minute driver-assigned free window.
		CreatedAt: pgtype.Timestamptz{Time: now.Add(-30 * time.Minute), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now.Add(-30 * time.Second), Valid: true},
	}
	policy := DefaultCancellationPolicy()

	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, policy, now)
	if err != nil {
		t.Errorf("ValidateCancellation() error = %v", err)
	}
	if result.FeeCents != 0 {
		t.Errorf("fee = %d, want 0 inside driver-assigned free window", result.FeeCents)
	}
}

func TestValidateCancellation_AlreadyCancelled(t *testing.T) {
	ride := db.Ride{ID: uuid.New(), Status: "cancelled"}
	result, err := ValidateCancellation(ride, "rider", CancellationReasonRiderRequested, DefaultCancellationPolicy(), time.Now())
	if err == nil {
		t.Error("ValidateCancellation() should fail for already-cancelled ride")
	}
	if result.Success {
		t.Error("ValidateCancellation() should not succeed for already-cancelled ride")
	}
}

func TestProcessNoShow_NoDriver(t *testing.T) {
	ride := db.Ride{ID: uuid.New(), Status: "requested"}
	_, err := ProcessNoShow(ride, "driver", DefaultCancellationPolicy(), time.Now())
	if err == nil {
		t.Error("ProcessNoShow() should fail without an assigned driver")
	}
}

func TestProcessNoShow_DriverNoShowIsFreeForRider(t *testing.T) {
	now := time.Now()
	ride := db.Ride{
		ID:     uuid.New(),
		Status: "driver_assigned",
		DriverID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		UpdatedAt: pgtype.Timestamptz{Time: now.Add(-6 * time.Minute), Valid: true},
	}
	policy := DefaultCancellationPolicy()

	result, err := ProcessNoShow(ride, "rider", policy, now)
	if err != nil {
		t.Errorf("ProcessNoShow() error = %v", err)
	}
	if !result.Success {
		t.Error("ProcessNoShow() should succeed after timeout")
	}
	if result.Reason != CancellationReasonDriverNoShow {
		t.Errorf("reason = %q, want %q", result.Reason, CancellationReasonDriverNoShow)
	}
	if result.FeeCents != 0 {
		t.Errorf("fee = %d, want 0 when the driver is the no-show party", result.FeeCents)
	}
}
