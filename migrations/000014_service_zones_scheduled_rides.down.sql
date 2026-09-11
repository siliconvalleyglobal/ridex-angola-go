DROP INDEX IF EXISTS idx_rides_requested_pickup;
DROP INDEX IF EXISTS idx_rides_service_zone;
ALTER TABLE rides
    DROP COLUMN IF EXISTS requested_pickup_at,
    DROP COLUMN IF EXISTS service_zone_id;
DROP INDEX IF EXISTS idx_service_zones_active;
DROP TABLE IF EXISTS service_zones;
