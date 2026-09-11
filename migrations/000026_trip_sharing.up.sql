-- Trip Sharing System
CREATE TABLE IF NOT EXISTS trip_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ride_id UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    rider_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMPTZ
);

CREATE INDEX idx_trip_shares_ride ON trip_shares(ride_id);
CREATE INDEX idx_trip_shares_token ON trip_shares(token_hash);
CREATE INDEX idx_trip_shares_active ON trip_shares(active, expires_at);

-- Trip Share Viewers (emergency contacts who can view)
CREATE TABLE IF NOT EXISTS trip_share_viewers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_share_id UUID NOT NULL REFERENCES trip_shares(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES emergency_contacts(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(trip_share_id, contact_id)
);

CREATE INDEX idx_trip_share_viewers ON trip_share_viewers(trip_share_id);
