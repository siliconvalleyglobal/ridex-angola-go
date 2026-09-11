DROP INDEX IF EXISTS idx_rides_idempotency_hash;
DROP INDEX IF EXISTS idx_rides_rider_idempotency;
ALTER TABLE rides
    DROP COLUMN IF EXISTS idempotency_payload_hash,
    DROP COLUMN IF EXISTS idempotency_key;
