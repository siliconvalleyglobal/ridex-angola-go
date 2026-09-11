package airport

import (
	"context"
	"fmt"
)

// FlightStatus represents the status of a flight.
type FlightStatus string

const (
	FlightStatusScheduled FlightStatus = "scheduled"
	FlightStatusBoarding  FlightStatus = "boarding"
	FlightStatusDeparted  FlightStatus = "departed"
	FlightStatusInFlight  FlightStatus = "in_flight"
	FlightStatusLanded    FlightStatus = "landed"
	FlightStatusDelayed   FlightStatus = "delayed"
	FlightStatusCancelled FlightStatus = "cancelled"
)

// FlightInfo represents flight information.
type FlightInfo struct {
	FlightNumber     string       `json:"flightNumber"`
	Airline          string       `json:"airline"`
	ScheduledArrival string       `json:"scheduledArrival"`
	EstimatedArrival string       `json:"estimatedArrival,omitempty"`
	Status           FlightStatus `json:"status"`
}

// FlightProvider is an interface for flight status providers.
type FlightProvider interface {
	GetFlightStatus(ctx context.Context, flightNumber string) (*FlightInfo, error)
}

// MockFlightProvider is a mock implementation for testing.
type MockFlightProvider struct {
	Flights map[string]*FlightInfo
}

// NewMockFlightProvider creates a new mock flight provider.
func NewMockFlightProvider() *MockFlightProvider {
	return &MockFlightProvider{
		Flights: make(map[string]*FlightInfo),
	}
}

// GetFlightStatus returns flight status from the mock provider.
func (m *MockFlightProvider) GetFlightStatus(ctx context.Context, flightNumber string) (*FlightInfo, error) {
	if flight, ok := m.Flights[flightNumber]; ok {
		return flight, nil
	}
	return nil, fmt.Errorf("flight %s not found", flightNumber)
}

// AddFlight adds a flight to the mock provider.
func (m *MockFlightProvider) AddFlight(flight *FlightInfo) {
	m.Flights[flight.FlightNumber] = flight
}
