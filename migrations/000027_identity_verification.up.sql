-- Identity Verification System
CREATE TABLE IF NOT EXISTS identity_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected', 'expired')),
    id_document_type VARCHAR(20) CHECK (id_document_type IN ('bi', 'passport', 'drivers_license')),
    id_document_url TEXT,
    selfie_url TEXT,
    verified_at TIMESTAMPTZ,
    verified_by UUID REFERENCES users(id) ON DELETE SET NULL,
    confidence_score DECIMAL(5,2),
    rejection_reason TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_identity_user ON identity_verifications(user_id);
CREATE INDEX idx_identity_status ON identity_verifications(status);
CREATE INDEX idx_identity_expires ON identity_verifications(expires_at);

-- Driver Badges
CREATE TABLE IF NOT EXISTS driver_badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    badge_type VARCHAR(30) NOT NULL CHECK (badge_type IN ('verified', 'top_rated', 'safety_champion', 'million_miles', 'early_adopter', 'community_helper')),
    display_name VARCHAR(50) NOT NULL,
    description TEXT,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_driver_badges_driver ON driver_badges(driver_id);
CREATE INDEX idx_driver_badges_type ON driver_badges(badge_type);
CREATE INDEX idx_driver_badges_active ON driver_badges(active);

-- Trigger to update timestamp
CREATE OR REPLACE FUNCTION update_identity_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_identity_timestamp
    BEFORE UPDATE ON identity_verifications
    FOR EACH ROW
    EXECUTE FUNCTION update_identity_timestamp();
