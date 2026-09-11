CREATE TABLE cash_payments (
    ride_id       UUID PRIMARY KEY REFERENCES rides(id) ON DELETE CASCADE,
    driver_id     UUID NOT NULL REFERENCES users(id),
    amount_due    NUMERIC(12,0) NOT NULL CHECK (amount_due > 0),
    amount_paid   NUMERIC(12,0) NOT NULL CHECK (amount_paid >= amount_due),
    change_cents  NUMERIC(12,0) NOT NULL CHECK (change_cents >= 0),
    confirmed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
