DROP INDEX IF EXISTS idx_payment_events_charge_provider_event;
DROP INDEX IF EXISTS idx_payment_charges_ride_idempotency;

ALTER TABLE payment_events
    DROP COLUMN IF EXISTS provider_event_id;

ALTER TABLE payment_charges
    DROP COLUMN IF EXISTS idempotency_key;
