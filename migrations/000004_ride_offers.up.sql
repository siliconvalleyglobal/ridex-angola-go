CREATE TABLE ride_offers (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id     UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    driver_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('offered', 'accepted', 'declined', 'expired')),
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_ride_offers_active_ride
    ON ride_offers(ride_id) WHERE status = 'offered';
CREATE INDEX idx_ride_offers_driver_status
    ON ride_offers(driver_id, status);
