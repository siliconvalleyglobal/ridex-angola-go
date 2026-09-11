package rides

import (
	"testing"
	"time"
)

func TestParseRequestedPickupAt(t *testing.T) {
	now := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
	got, err := parseRequestedPickupAt("2026-09-10T06:00:00+01:00", now)
	if err != nil {
		t.Fatalf("parseRequestedPickupAt() error = %v", err)
	}
	if !got.Equal(time.Date(2026, 9, 10, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("pickup time = %v, want UTC-normalized time", got)
	}
	if _, err := parseRequestedPickupAt("2026-09-10T03:59:59Z", now); err == nil {
		t.Fatal("past pickup time should be rejected")
	}
	if _, err := parseRequestedPickupAt("not-a-time", now); err == nil {
		t.Fatal("malformed pickup time should be rejected")
	}
}
