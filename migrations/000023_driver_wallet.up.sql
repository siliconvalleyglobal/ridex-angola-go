-- Driver Wallet System
CREATE TABLE IF NOT EXISTS driver_wallets (
    driver_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0,
    pending_cents BIGINT NOT NULL DEFAULT 0,
    total_earned_cents BIGINT NOT NULL DEFAULT 0,
    total_paid_cents BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'AOA',
    last_payout_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('earning', 'payout', 'bonus', 'penalty', 'adjustment')),
    amount_cents BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reference_id UUID,
    reference_type VARCHAR(20),
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'failed', 'reversed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wallet_transactions_driver ON wallet_transactions(driver_id, created_at DESC);
CREATE INDEX idx_wallet_transactions_type ON wallet_transactions(type);
CREATE INDEX idx_wallet_transactions_reference ON wallet_transactions(reference_id);

-- Payout Requests
CREATE TABLE IF NOT EXISTS payout_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    method VARCHAR(20) NOT NULL CHECK (method IN ('bank', 'mobile_money', 'multicada')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    reference_id VARCHAR(100),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payout_requests_driver ON payout_requests(driver_id, created_at DESC);
CREATE INDEX idx_payout_requests_status ON payout_requests(status);

-- Payout Methods
CREATE TABLE IF NOT EXISTS payout_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('bank', 'mobile_money', 'multicada')),
    account_name VARCHAR(120),
    account_number VARCHAR(50),
    bank_name VARCHAR(100),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payout_methods_driver ON payout_methods(driver_id);

-- Trigger to update wallet timestamp
CREATE OR REPLACE FUNCTION update_wallet_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_wallet_timestamp
    BEFORE UPDATE ON driver_wallets
    FOR EACH ROW
    EXECUTE FUNCTION update_wallet_timestamp();
