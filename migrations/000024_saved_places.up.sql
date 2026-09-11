-- Saved Places for Quick Booking
CREATE TABLE IF NOT EXISTS saved_places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    address TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    icon VARCHAR(20) NOT NULL DEFAULT 'other' CHECK (icon IN ('home', 'work', 'gym', 'school', 'airport', 'hospital', 'other')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_saved_places_user ON saved_places(user_id);
CREATE INDEX idx_saved_places_icon ON saved_places(icon);

-- Trigger to update timestamp
CREATE OR REPLACE FUNCTION update_saved_place_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_saved_place_timestamp
    BEFORE UPDATE ON saved_places
    FOR EACH ROW
    EXECUTE FUNCTION update_saved_place_timestamp();
