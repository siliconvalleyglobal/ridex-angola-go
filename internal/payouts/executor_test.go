package payouts

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestManualExecutorSubmitsAndTracks(t *testing.T) {
	exec := NewManualExecutor()
	driverID := uuid.New()

	submission, err := exec.Submit("bank", 50000, driverID, "req-1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !strings.HasPrefix(submission.Reference, "payout-") {
		t.Fatalf("submission reference = %q, want payout- prefix", submission.Reference)
	}
	if submission.Status != StatusProcessing {
		t.Fatalf("submission status = %q, want processing", submission.Status)
	}
	if !strings.Contains(submission.Message, "manual") {
		t.Fatalf("submission message = %q, want manual executor", submission.Message)
	}

	info, err := exec.Status(submission.Reference)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if info.Status != StatusProcessing {
		t.Fatalf("status = %q, want processing", info.Status)
	}

	exec.Record(submission.Reference, StatusCompleted)
	info, err = exec.Status(submission.Reference)
	if err != nil || info.Status != StatusCompleted {
		t.Fatalf("status after Record = %q err = %v, want completed", info.Status, err)
	}
}

func TestManualExecutorValidatesInput(t *testing.T) {
	exec := NewManualExecutor()
	driverID := uuid.New()

	if _, err := exec.Submit("", 50000, driverID, "r"); err == nil {
		t.Fatal("empty method accepted")
	}
	if _, err := exec.Submit("bank", 0, driverID, "r"); err == nil {
		t.Fatal("zero amount accepted")
	}
	if _, err := exec.Submit("bank", -1, driverID, "r"); err == nil {
		t.Fatal("negative amount accepted")
	}
	if _, err := exec.Submit("bank", 50000, uuid.Nil, "r"); err == nil {
		t.Fatal("nil driver accepted")
	}
	if _, err := exec.Status("never-submitted"); !strings.Contains(err.Error(), "unknown payout reference") {
		t.Fatalf("unknown reference error = %v, want unknown payout reference", err)
	}
}
