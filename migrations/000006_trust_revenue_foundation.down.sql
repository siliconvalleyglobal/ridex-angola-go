DROP TABLE IF EXISTS driver_kyc;
ALTER TABLE rides
    DROP COLUMN IF EXISTS trip_pin_verified,
    DROP COLUMN IF EXISTS trip_pin_hash,
    DROP COLUMN IF EXISTS payment_method;
