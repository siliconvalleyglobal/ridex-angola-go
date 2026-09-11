-- OTPs are never stored in plaintext. Existing verification rows are
-- migrated in place; old rows are invalidated by clearing their code hash.
ALTER TABLE verification_codes
    RENAME COLUMN code TO code_hash;

ALTER TABLE verification_codes
    DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;

ALTER TABLE verification_codes
    ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('login', 'registration', 'phone_change', 'password_reset'));

ALTER TABLE verification_codes
    ADD COLUMN verified_at TIMESTAMPTZ;

ALTER TABLE verification_codes
    ALTER COLUMN code_hash DROP NOT NULL;

UPDATE verification_codes
SET code_hash = NULL,
    expires_at = NOW()
WHERE code_hash IS NOT NULL;

CREATE INDEX idx_verification_codes_active
    ON verification_codes(user_id, purpose, created_at DESC)
    WHERE verified_at IS NULL;

CREATE TABLE emergency_contacts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
    phone           TEXT NOT NULL CHECK (char_length(phone) BETWEEN 7 AND 30),
    relationship    TEXT NOT NULL CHECK (char_length(relationship) BETWEEN 2 AND 60),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, phone)
);

CREATE INDEX idx_emergency_contacts_user_id
    ON emergency_contacts(user_id, created_at, id);

CREATE TABLE ride_sos_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    triggered_by    UUID NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'resolved')),
    location        POINT,
    note            TEXT CHECK (note IS NULL OR char_length(note) <= 500),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id)
);

CREATE INDEX idx_ride_sos_events_ride_id
    ON ride_sos_events(ride_id, created_at DESC);

CREATE TABLE ride_incident_reports (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    reported_by     UUID NOT NULL REFERENCES users(id),
    category        TEXT NOT NULL CHECK (category IN (
        'accident', 'harassment', 'unsafe_driving', 'vehicle_issue',
        'payment_dispute', 'other'
    )),
    description     TEXT NOT NULL CHECK (char_length(description) BETWEEN 1 AND 4000),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ride_incident_reports_ride_id
    ON ride_incident_reports(ride_id, created_at DESC);
