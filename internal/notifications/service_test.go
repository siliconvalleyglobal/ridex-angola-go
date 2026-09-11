package notifications

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/db"
)

func TestPreferenceEnabled(t *testing.T) {
	prefs := db.NotificationPreference{
		RideUpdates: true, PaymentUpdates: false, KycUpdates: true, Marketing: false,
	}
	tests := []struct {
		name string
		kind string
		want bool
	}{
		{"ride", "ride_status", true},
		{"payment", "payment_status", false},
		{"kyc", "kyc_update", true},
		{"marketing", "promotion", false},
		{"unknown", "safety_alert", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := preferenceEnabled(prefs, test.kind); got != test.want {
				t.Fatalf("preferenceEnabled(%q) = %v, want %v", test.kind, got, test.want)
			}
		})
	}
}

func TestDefaultPreferences(t *testing.T) {
	userID := uuid.New()
	got := defaultPreferences(userID)
	if got.UserID != userID || !got.RideUpdates || !got.PaymentUpdates || !got.KycUpdates || got.Marketing {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}
