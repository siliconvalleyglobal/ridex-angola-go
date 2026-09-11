ALTER TABLE rides
    ADD COLUMN payment_method TEXT NOT NULL DEFAULT 'cash'
        CHECK (payment_method IN ('cash', 'multicaixa', 'card')),
    ADD COLUMN trip_pin_hash TEXT,
    ADD COLUMN trip_pin_verified BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE driver_kyc (
    driver_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    id_document     TEXT NOT NULL,
    license_number  TEXT NOT NULL,
    vehicle_plate   TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'under_review', 'approved', 'rejected')),
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at     TIMESTAMPTZ,
    review_note     TEXT
);
