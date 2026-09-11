# Airport Transfers for RideX Angola

## Overview

This module provides airport transfer functionality for RideX Angola, starting with **4 de Fevereiro International Airport (LAD)** in Luanda. It supports:

- Airport pickup and dropoff ride booking
- Flight number tracking
- Meet-and-greet services
- Waiting time charges
- Fixed airport fares
- Multi-terminal pickup zones

## Features

### Airport Management

- **Airport Registry**: Store airport details (IATA code, name, location, timezone, terminals)
- **Pickup Zones**: Define designated pickup areas at each airport
- **Multi-Airport Support**: Can support multiple airports (future expansion)

### Airport Transfer Booking

- **Flight Integration**: Track flight numbers and scheduled arrivals
- **Passenger Info**: Store passenger name and contact details
- **Meet-and-Greet**: Optional premium service with driver meeting passenger
- **Waiting Time**: Configurable waiting period with per-minute charges
- **Fixed Fares**: Special pricing for airport transfers

### Transfer Lifecycle

1. **Pending**: Transfer booked but not yet assigned
2. **Assigned**: Driver assigned to the transfer
3. **EnRoute**: Driver en route to pickup location
4. **Arrived**: Driver arrived at pickup location
5. **Completed**: Transfer completed successfully
6. **Cancelled**: Transfer cancelled (with reason)
7. **NoShow**: Passenger didn't show up

## API Endpoints

### Airports

- `GET /api/v1/airports` - List all airports
- `GET /api/v1/airports/:id` - Get airport by ID
- `GET /api/v1/airports/iata/:code` - Get airport by IATA code

### Airport Transfers

- `POST /api/v1/airport-transfers` - Create airport transfer
- `GET /api/v1/airport-transfers` - List airport transfers
- `GET /api/v1/airport-transfers/:id` - Get transfer by ID
- `GET /api/v1/airport-transfers/ride/:ride_id` - Get transfer by ride ID
- `PATCH /api/v1/airport-transfers/:id` - Update transfer
- `POST /api/v1/airport-transfers/:id/arrive` - Confirm driver arrival
- `POST /api/v1/airport-transfers/:id/complete` - Complete transfer
- `POST /api/v1/airport-transfers/:id/cancel` - Cancel transfer

## Data Structures

### Airport

```json
{
  "id": "uuid",
  "iata": "LAD",
  "name": "4 de Fevereiro International Airport",
  "city": "Luanda",
  "country": "Angola",
  "country_code": "AO",
  "latitude": -8.8426,
  "longitude": 13.2346,
  "timezone": "Africa/Luanda",
  "terminal_count": 1,
  "pickup_zones": ["airport_terminal", "arrivals_hall", ...]
}
```

### Airport Transfer Request

```json
{
  "ride_id": "uuid",
  "airport_id": "uuid",
  "is_pickup": true,
  "flight_number": "TA123",
  "airline": "TAAG",
  "scheduled_arrival": "2024-01-01T10:00:00Z",
  "passenger_name": "João Silva",
  "passenger_phone": "+244 912 123 456",
  "meet_and_greet": true,
  "waiting_minutes": 30,
  "notes": "Looking for driver with sign"
}
```

### Airport Transfer Response

```json
{
  "id": "uuid",
  "ride_id": "uuid",
  "airport_id": "uuid",
  "airport": { ... },
  "is_pickup": true,
  "flight_number": "TA123",
  "airline": "TAAG",
  "scheduled_arrival": "2024-01-01T10:00:00Z",
  "actual_arrivial": null,
  "passenger_name": "João Silva",
  "passenger_phone": "+244 912 123 456",
  "meet_and_greet": true,
  "waiting_minutes": 30,
  "waiting_fee_cents": 3000,
  "fixed_fare_cents": 2000,
  "status": "pending",
  "notes": "Looking for driver with sign"
}
```

## Pricing

### Base Airport Fare
- **Minimum fare**: 2,000 AOA
- **Per km**: 100 AOA

### Waiting Time (Meet-and-Greet)
- **Rate**: 100 AOA per minute
- **Minimum wait**: 15 minutes
- **Maximum wait**: 60 minutes (after that, cancellation may apply)

### Example Calculation

For a 25 km airport transfer with 30 minutes waiting:
- Base fare: 2,000 AOA
- Distance: 25 × 100 = 2,500 AOA
- Waiting: 30 × 100 = 3,000 AOA
- **Total**: 7,500 AOA

## 4 de Fevereiro Airport (LAD)

### Location
- **Coordinates**: -8.8426, 13.2346
- **Timezone**: Africa/Luanda (WAT, UTC+1)
- **Terminals**: 1

### Pickup Zones
1. **airport_terminal** - Main terminal pickup
2. **arrivals_hall** - Arrivals hall pickup
3. **departures_hall** - Departures hall (for dropoffs)
4. **parking_level_1** - Lower parking level
5. **parking_level_2** - Upper parking level
6. **meet_greet_area** - Dedicated meet-and-greet area

### Operational Notes
- Traffic can be heavy during rush hours (7-9 AM, 5-7 PM)
- Allow extra time for security checks
- Meet-and-greet recommended for international arrivals
- Payment can be cash or card

## Integration with Rides

Airport transfers are linked to regular rides:
- Each transfer has a `ride_id` referencing the ride
- Transfer status affects ride lifecycle
- Waiting fees are added to the ride fare
- Cancellation policies apply

## Future Enhancements

- [ ] Support for multiple airports (Lobito, Benguela, Huambo)
- [ ] Flight delay tracking integration
- [ ] Real-time flight status updates
- [ ] Airport queue management
- [ ] VIP/loyalty program integration
- [ ] Corporate account billing
- [ ] Multi-language support (Portuguese, English, French)
