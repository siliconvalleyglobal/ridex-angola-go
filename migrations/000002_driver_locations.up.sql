-- RideX Angola — Driver locations (live positions for matching)
-- One row per driver, upserted on each location ping.

CREATE TABLE driver_locations (
    driver_id  UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    location   POINT       NOT NULL,
    heading    NUMERIC(5,1),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_driver_locations_location ON driver_locations USING gist (location);

-- Only drivers with a recent ping are matchable; stale rows are cleaned by the worker.
CREATE INDEX idx_driver_locations_updated ON driver_locations(updated_at);
