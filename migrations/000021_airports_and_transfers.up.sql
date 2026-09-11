-- Airports and airport transfers migration

-- Create airports table
CREATE TABLE IF NOT EXISTS airports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iata VARCHAR(3) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    city VARCHAR(255) NOT NULL,
    country VARCHAR(255) NOT NULL,
    country_code VARCHAR(2) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    timezone VARCHAR(50) NOT NULL,
    terminal_count INTEGER NOT NULL DEFAULT 1,
    pickup_zones TEXT[],
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create airport_transfers table
CREATE TYPE airport_transfer_status AS ENUM (
    'pending',
    'assigned',
    'en_route',
    'arrived',
    'completed',
    'cancelled',
    'no_show'
);

CREATE TABLE IF NOT EXISTS airport_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ride_id UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    airport_id UUID NOT NULL REFERENCES airports(id) ON DELETE CASCADE,
    is_pickup BOOLEAN NOT NULL DEFAULT true,
    flight_number VARCHAR(10),
    airline VARCHAR(50),
    scheduled_arrival TIMESTAMP WITH TIME ZONE,
    actual_arrival TIMESTAMP WITH TIME ZONE,
    passenger_name VARCHAR(255),
    passenger_phone VARCHAR(50),
    meet_and_greet BOOLEAN NOT NULL DEFAULT false,
    waiting_minutes INTEGER NOT NULL DEFAULT 0,
    waiting_fee_cents INTEGER NOT NULL DEFAULT 0,
    fixed_fare_cents INTEGER NOT NULL DEFAULT 0,
    status airport_transfer_status NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_airports_iata ON airports(iata);
CREATE INDEX idx_airports_city ON airports(city);
CREATE INDEX idx_airport_transfers_ride_id ON airport_transfers(ride_id);
CREATE INDEX idx_airport_transfers_airport_id ON airport_transfers(airport_id);
CREATE INDEX idx_airport_transfers_status ON airport_transfers(status);
CREATE INDEX idx_airport_transfers_scheduled_arrival ON airport_transfers(scheduled_arrival);

-- Insert 4 de Fevereiro Airport (LAD)
INSERT INTO airports (iata, name, city, country, country_code, latitude, longitude, timezone, terminal_count, pickup_zones)
VALUES (
    'LAD',
    '4 de Fevereiro International Airport',
    'Luanda',
    'Angola',
    'AO',
    -8.8426,
    13.2346,
    'Africa/Luanda',
    1,
    ARRAY['airport_terminal', 'arrivals_hall', 'departures_hall', 'parking_level_1', 'parking_level_2']
);
