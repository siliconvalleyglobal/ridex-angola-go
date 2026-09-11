package airport

import (
	"time"

	"github.com/google/uuid"
)

// Airport represents an airport with its operational details.
type Airport struct {
	ID            uuid.UUID `json:"id"`
	IATA          string    `json:"iata"`
	Name          string    `json:"name"`
	City          string    `json:"city"`
	Country       string    `json:"country"`
	CountryCode   string    `json:"country_code"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timezone      string    `json:"timezone"`
	TerminalCount int       `json:"terminal_count"`
	PickupZones   []string  `json:"pickup_zones"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AirportTransfer represents a ride booked for airport pickup/dropoff.
type AirportTransfer struct {
	ID               uuid.UUID             `json:"id"`
	RideID           uuid.UUID             `json:"ride_id"`
	AirportID        uuid.UUID             `json:"airport_id"`
	Airport          *Airport              `json:"airport,omitempty"`
	IsPickup         bool                  `json:"is_pickup"`
	FlightNumber     string                `json:"flight_number,omitempty"`
	Airline          string                `json:"airline,omitempty"`
	ScheduledArrival *time.Time            `json:"scheduled_arrival,omitempty"`
	ActualArrival    *time.Time            `json:"actual_arrival,omitempty"`
	PassengerName    string                `json:"passenger_name,omitempty"`
	PassengerPhone   string                `json:"passenger_phone,omitempty"`
	MeetAndGreet     bool                  `json:"meet_and_greet"`
	WaitingMinutes   int                   `json:"waiting_minutes"`
	WaitingFeeCents  int64                 `json:"waiting_fee_cents"`
	FixedFareCents   int64                 `json:"fixed_fare_cents"`
	Status           AirportTransferStatus `json:"status"`
	Notes            string                `json:"notes,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// AirportTransferStatus represents the status of an airport transfer.
type AirportTransferStatus string

const (
	AirportTransferStatusPending   AirportTransferStatus = "pending"
	AirportTransferStatusAssigned  AirportTransferStatus = "assigned"
	AirportTransferStatusEnRoute   AirportTransferStatus = "en_route"
	AirportTransferStatusArrived   AirportTransferStatus = "arrived"
	AirportTransferStatusCompleted AirportTransferStatus = "completed"
	AirportTransferStatusCancelled AirportTransferStatus = "cancelled"
	AirportTransferStatusNoShow    AirportTransferStatus = "no_show"
)

// AirportTransferEvent represents an event in the airport transfer lifecycle.
type AirportTransferEvent struct {
	ID                uuid.UUID              `json:"id"`
	AirportTransferID uuid.UUID              `json:"airport_transfer_id"`
	EventType         string                 `json:"event_type"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
}

// AirportTransferCreateInput represents input for creating an airport transfer.
type AirportTransferCreateInput struct {
	RideID           uuid.UUID  `json:"ride_id"`
	AirportID        uuid.UUID  `json:"airport_id"`
	IsPickup         bool       `json:"is_pickup"`
	FlightNumber     string     `json:"flight_number,omitempty"`
	Airline          string     `json:"airline,omitempty"`
	ScheduledArrival *time.Time `json:"scheduled_arrival,omitempty"`
	PassengerName    string     `json:"passenger_name,omitempty"`
	PassengerPhone   string     `json:"passenger_phone,omitempty"`
	MeetAndGreet     bool       `json:"meet_and_greet"`
	WaitingMinutes   int        `json:"waiting_minutes"`
	WaitingFeeCents  int64      `json:"waiting_fee_cents"`
	FixedFareCents   int64      `json:"fixed_fare_cents"`
	Notes            string     `json:"notes,omitempty"`
}

// AirportTransferUpdateInput represents input for updating an airport transfer.
type AirportTransferUpdateInput struct {
	Status         *AirportTransferStatus `json:"status,omitempty"`
	ActualArrival  *time.Time             `json:"actual_arrival,omitempty"`
	WaitingMinutes *int                   `json:"waiting_minutes,omitempty"`
	Notes          *string                `json:"notes,omitempty"`
}
