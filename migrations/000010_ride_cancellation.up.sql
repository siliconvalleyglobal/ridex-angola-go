ALTER TABLE rides
    ADD COLUMN cancellation_reason TEXT,
    ADD COLUMN cancelled_by UUID REFERENCES users(id);

CREATE INDEX idx_rides_cancelled_by ON rides(cancelled_by)
    WHERE cancelled_by IS NOT NULL;
