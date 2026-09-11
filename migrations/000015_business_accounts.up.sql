-- Business accounts provide a small, provider-independent corporate billing
-- boundary. Members are normal users and retain their existing rider access.
CREATE TABLE business_accounts (
    id                         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id                   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name                       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    currency                   TEXT NOT NULL DEFAULT 'AOA',
    monthly_limit_cents        NUMERIC(12,0) CHECK (monthly_limit_cents IS NULL OR monthly_limit_cents >= 0),
    require_ride_approval      BOOLEAN NOT NULL DEFAULT false,
    is_active                  BOOLEAN NOT NULL DEFAULT true,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_business_accounts_owner ON business_accounts(owner_id);

CREATE TABLE business_members (
    account_id                 UUID NOT NULL REFERENCES business_accounts(id) ON DELETE CASCADE,
    user_id                    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role                       TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    spending_limit_cents       NUMERIC(12,0) CHECK (spending_limit_cents IS NULL OR spending_limit_cents >= 0),
    status                     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, user_id)
);

CREATE INDEX idx_business_members_user ON business_members(user_id);

CREATE TABLE ride_authorizations (
    id                         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id                    UUID NOT NULL UNIQUE REFERENCES rides(id) ON DELETE CASCADE,
    account_id                 UUID NOT NULL REFERENCES business_accounts(id) ON DELETE CASCADE,
    member_id                  UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status                     TEXT NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending', 'approved', 'rejected')),
    requested_amount_cents     NUMERIC(12,0) NOT NULL CHECK (requested_amount_cents >= 0),
    authorized_by              UUID REFERENCES users(id) ON DELETE SET NULL,
    decision_note              TEXT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at                 TIMESTAMPTZ
);

CREATE INDEX idx_ride_authorizations_account_status ON ride_authorizations(account_id, status);
CREATE INDEX idx_ride_authorizations_pending ON ride_authorizations(status) WHERE status = 'pending';

ALTER TABLE rides
    ADD COLUMN business_account_id UUID REFERENCES business_accounts(id) ON DELETE SET NULL,
    ADD COLUMN business_member_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_rides_business_account ON rides(business_account_id, created_at);
