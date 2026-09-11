CREATE TABLE driver_availability (
    driver_id   UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    is_online   BOOLEAN NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_driver_availability_online
    ON driver_availability(is_online, updated_at)
    WHERE is_online;
