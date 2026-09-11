package safety

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

func TestCanAccessRideOnlyAllowsParticipants(t *testing.T) {
	rider := uuid.New()
	driver := uuid.New()
	other := uuid.New()
	ride := db.Ride{
		RiderID:  rider,
		DriverID: pgtype.UUID{Bytes: driver, Valid: true},
	}

	tests := []struct {
		name string
		id   uuid.UUID
		role string
		want bool
	}{
		{"rider owner", rider, "rider", true},
		{"assigned driver", driver, "driver", true},
		{"other rider", other, "rider", false},
		{"other driver", other, "driver", false},
		{"admin cannot trigger", other, "admin", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccessRide(tt.id, tt.role, ride); got != tt.want {
				t.Fatalf("CanAccessRide() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanAccessRideRejectsUnassignedDriver(t *testing.T) {
	ride := db.Ride{RiderID: uuid.New()}
	if CanAccessRide(uuid.New(), "driver", ride) {
		t.Fatal("unassigned driver was authorized")
	}
}
