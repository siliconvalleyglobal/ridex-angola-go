-- Request keys are scoped to the authenticated rider. The payload hash
-- prevents accidental reuse of a key for a different ride request.
ALTER TABLE rides
    ADD COLUMN idempotency_key TEXT,
    ADD COLUMN idempotency_payload_hash TEXT;

CREATE UNIQUE INDEX idx_rides_rider_idempotency
    ON rides(rider_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_rides_idempotency_hash
    ON rides(idempotency_payload_hash)
    WHERE idempotency_payload_hash IS NOT NULL;
