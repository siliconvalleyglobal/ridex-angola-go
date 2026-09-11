-- name: SetDriverAvailability :one
INSERT INTO driver_availability (driver_id, is_online, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (driver_id)
DO UPDATE SET is_online = EXCLUDED.is_online, updated_at = NOW()
RETURNING driver_id, is_online, updated_at;

-- name: GetDriverAvailability :one
SELECT driver_id, is_online, updated_at
FROM driver_availability
WHERE driver_id = $1;
