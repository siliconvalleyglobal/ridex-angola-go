DROP INDEX IF EXISTS idx_rides_cancelled_by;

ALTER TABLE rides
    DROP COLUMN IF EXISTS cancelled_by,
    DROP COLUMN IF EXISTS cancellation_reason;
