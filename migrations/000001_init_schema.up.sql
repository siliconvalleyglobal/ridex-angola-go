-- RideX Angola — Initial Schema
-- PostGIS-independent: uses Postgres built-in point type for geo.
-- Append-only by design: state lives in ride_events, not mutated on rides rows.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Users ──────────────────────────────────────────────────────────────
CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone           TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    role            TEXT        NOT NULL CHECK (role IN ('rider', 'driver')),
    password_hash   TEXT        NOT NULL,
    rating          NUMERIC(3,2) DEFAULT 0.00 CHECK (rating >= 0 AND rating <= 5),
    acceptance_rate NUMERIC(5,2) DEFAULT 0.00 CHECK (acceptance_rate >= 0 AND acceptance_rate <= 100),
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_phone ON users(phone);
CREATE INDEX idx_users_role_active ON users(role, is_active) WHERE is_active;

-- ── Sessions (refresh tokens stored in Valkey; this table is the audit ledger) ──
CREATE TABLE sessions (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id       TEXT        NOT NULL,
    ip_address      TEXT        NOT NULL,
    user_agent      TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- ── Verification codes (OTP ledger) ───────────────────────────────────
CREATE TABLE verification_codes (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code            TEXT        NOT NULL,
    purpose         TEXT        NOT NULL CHECK (purpose IN ('login', 'registration', 'phone_change')),
    attempts        INTEGER     NOT NULL DEFAULT 0,
    max_attempts    INTEGER     NOT NULL DEFAULT 5,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_verification_codes_user_id ON verification_codes(user_id);
CREATE INDEX idx_verification_codes_expires ON verification_codes(expires_at) WHERE expires_at > 'epoch';

-- ── Rides ─────────────────────────────────────────────────────────────
-- Uses Postgres built-in point type (no PostGIS required).
CREATE TABLE rides (
    id                  UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    rider_id            UUID        NOT NULL REFERENCES users(id),
    driver_id           UUID        REFERENCES users(id),
    pickup_point        POINT       NOT NULL,
    destination_point   POINT       NOT NULL,
    pickup_address      TEXT        NOT NULL,
    destination_address TEXT        NOT NULL,
    status              TEXT        NOT NULL CHECK (
        status IN ('requested', 'matched', 'driver_arriving', 'in_progress', 'completed', 'cancelled')
    ),
    suggested_fare_cents NUMERIC(12,0) NOT NULL,
    accepted_fare_cents NUMERIC(12,0),
    currency            TEXT        NOT NULL DEFAULT 'AOA',
    promo_code          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ
);
CREATE INDEX idx_rides_rider_id ON rides(rider_id);
CREATE INDEX idx_rides_driver_id ON rides(driver_id);
CREATE INDEX idx_rides_status ON rides(status);
CREATE INDEX idx_rides_created ON rides(created_at);

-- ── Ride events (append-only ledger) ──────────────────────────────────
CREATE TABLE ride_events (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id     UUID        NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL,
    payload     JSONB       NOT NULL DEFAULT '{}',
    created_by  UUID        REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ride_events_ride_id ON ride_events(ride_id);
CREATE INDEX idx_ride_events_type ON ride_events(event_type);
CREATE INDEX idx_ride_events_created ON ride_events(created_at);

-- ── Payment charges ────────────────────────────────────────────────────
CREATE TABLE payment_charges (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID        NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    provider        TEXT        NOT NULL CHECK (provider IN ('appypay', 'vpos', 'proxypay')),
    provider_charge_id TEXT     NOT NULL,
    amount_cents    NUMERIC(12,0) NOT NULL,
    currency        TEXT        NOT NULL DEFAULT 'AOA',
    status          TEXT        NOT NULL CHECK (
        status IN ('pending', 'processing', 'completed', 'failed', 'refunded')
    ),
    raw_response    JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    refunded_at     TIMESTAMPTZ
);

CREATE INDEX idx_payment_charges_ride_id ON payment_charges(ride_id);
CREATE INDEX idx_payment_charges_provider ON payment_charges(provider);
CREATE INDEX idx_payment_charges_status ON payment_charges(status);

-- ── Payment events (append-only ledger) ────────────────────────────────
CREATE TABLE payment_events (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    charge_id   UUID        NOT NULL REFERENCES payment_charges(id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL,
    payload     JSONB       NOT NULL DEFAULT '{}',
    signature   TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_events_charge_id ON payment_events(charge_id);

-- ── SAFT invoices ──────────────────────────────────────────────────────
CREATE TABLE saft_invoices (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID        NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    charge_id       UUID        REFERENCES payment_charges(id),
    invoice_number  TEXT        NOT NULL UNIQUE,
    issue_date      DATE        NOT NULL,
    due_date        DATE        NOT NULL,
    customer_name   TEXT        NOT NULL,
    customer_tax_id TEXT,
    items           JSONB       NOT NULL DEFAULT '[]',
    total_cents     NUMERIC(12,0) NOT NULL,
    currency        TEXT        NOT NULL DEFAULT 'AOA',
    status          TEXT        NOT NULL CHECK (status IN ('draft', 'issued', 'paid', 'cancelled')),
    pdf_content     BYTEA,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    issued_at       TIMESTAMPTZ,
    paid_at         TIMESTAMPTZ
);

CREATE INDEX idx_saft_invoices_ride_id ON saft_invoices(ride_id);
CREATE INDEX idx_saft_invoices_number ON saft_invoices(invoice_number);

-- ── Config / constants (small reference data) ─────────────────────────
CREATE TABLE config (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO config (key, value) VALUES
    ('fare_base_cents', '1000'),
    ('fare_per_km_cents', '500'),
    ('fare_per_min_cents', '30'),
    ('matching_radius_meters', '5000'),
    ('offer_timeout_sec', '15'),
    ('currency_code', 'AOA'),
    ('currency_symbol', 'AOA'),
    ('language', 'pt-AO');
