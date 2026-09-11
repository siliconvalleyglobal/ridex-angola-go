-- Promo Codes and Loyalty Program
CREATE TABLE IF NOT EXISTS promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed')),
    discount_cents BIGINT NOT NULL DEFAULT 0,
    max_uses INTEGER NOT NULL DEFAULT 1,
    current_uses INTEGER NOT NULL DEFAULT 0,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ NOT NULL,
    min_ride_cents BIGINT NOT NULL DEFAULT 0,
    max_discount_cents BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_promo_codes_code ON promo_codes(code);
CREATE INDEX idx_promo_codes_active ON promo_codes(is_active, valid_from, valid_until);

-- Track promo code usage per user
CREATE TABLE IF NOT EXISTS promo_code_usages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ride_id UUID REFERENCES rides(id) ON DELETE SET NULL,
    discount_cents BIGINT NOT NULL,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(promo_id, user_id, ride_id)
);

CREATE INDEX idx_promo_usage_user ON promo_code_usages(user_id);
CREATE INDEX idx_promo_usage_promo ON promo_code_usages(promo_id);

-- Loyalty Points
CREATE TABLE IF NOT EXISTS loyalty_points (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT NOT NULL DEFAULT 0,
    total_earned BIGINT NOT NULL DEFAULT 0,
    total_redeemed BIGINT NOT NULL DEFAULT 0,
    tier VARCHAR(20) NOT NULL DEFAULT 'bronze' CHECK (tier IN ('bronze', 'silver', 'gold', 'platinum')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loyalty_tier ON loyalty_points(tier);

-- Loyalty Transactions
CREATE TABLE IF NOT EXISTS loyalty_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    points BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('earn', 'redeem', 'expire', 'bonus')),
    reason VARCHAR(200),
    reference_id UUID,
    reference_type VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loyalty_transactions_user ON loyalty_transactions(user_id, created_at DESC);
CREATE INDEX idx_loyalty_transactions_type ON loyalty_transactions(type);

-- Trigger to update timestamps
CREATE OR REPLACE FUNCTION update_promo_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_promo_timestamp
    BEFORE UPDATE ON promo_codes
    FOR EACH ROW
    EXECUTE FUNCTION update_promo_timestamp();

CREATE TRIGGER trigger_update_loyalty_timestamp
    BEFORE UPDATE ON loyalty_points
    FOR EACH ROW
    EXECUTE FUNCTION update_promo_timestamp();
