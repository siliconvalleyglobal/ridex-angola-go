-- Add provider-neutral idempotency keys without introducing provider-specific
-- request or response fields. Existing payment rows remain valid.
ALTER TABLE payment_charges
    ADD COLUMN idempotency_key TEXT;

CREATE UNIQUE INDEX idx_payment_charges_ride_idempotency
    ON payment_charges(ride_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

ALTER TABLE payment_events
    ADD COLUMN provider_event_id TEXT;

CREATE UNIQUE INDEX idx_payment_events_charge_provider_event
    ON payment_events(charge_id, provider_event_id)
    WHERE provider_event_id IS NOT NULL;
