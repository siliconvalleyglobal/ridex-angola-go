-- Service coverage rectangles and scheduled pickup support.
-- Rectangles are deliberately used instead of PostGIS so the local deployment
-- remains dependency-free and deterministic.

CREATE TABLE service_zones (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug           TEXT NOT NULL UNIQUE CHECK (char_length(slug) BETWEEN 1 AND 80),
    min_lat        NUMERIC(9,6) NOT NULL CHECK (min_lat BETWEEN -90 AND 90),
    min_lng        NUMERIC(9,6) NOT NULL CHECK (min_lng BETWEEN -180 AND 180),
    max_lat        NUMERIC(9,6) NOT NULL CHECK (max_lat BETWEEN -90 AND 90),
    max_lng        NUMERIC(9,6) NOT NULL CHECK (max_lng BETWEEN -180 AND 180),
    is_active      BOOLEAN NOT NULL DEFAULT true,
    base_cents     NUMERIC(12,0) CHECK (base_cents IS NULL OR base_cents >= 0),
    per_km_cents   NUMERIC(12,0) CHECK (per_km_cents IS NULL OR per_km_cents >= 0),
    per_min_cents  NUMERIC(12,0) CHECK (per_min_cents IS NULL OR per_min_cents >= 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_zones_latitude_order CHECK (min_lat < max_lat),
    CONSTRAINT service_zones_longitude_order CHECK (min_lng < max_lng)
);

CREATE INDEX idx_service_zones_active ON service_zones(is_active);

ALTER TABLE rides
    ADD COLUMN service_zone_id UUID REFERENCES service_zones(id) ON DELETE SET NULL,
    ADD COLUMN requested_pickup_at TIMESTAMPTZ;

CREATE INDEX idx_rides_service_zone ON rides(service_zone_id);
CREATE INDEX idx_rides_requested_pickup ON rides(requested_pickup_at)
    WHERE status = 'requested' AND driver_id IS NULL;
