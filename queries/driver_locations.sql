-- name: UpsertDriverLocation :one
INSERT INTO driver_locations (driver_id, location, heading, updated_at)
VALUES ($1, $2::point, $3, NOW())
ON CONFLICT (driver_id)
DO UPDATE SET location = EXCLUDED.location, heading = EXCLUDED.heading, updated_at = NOW()
RETURNING driver_id, location, heading, updated_at;

-- name: GetDriverLocation :one
SELECT driver_id, location, heading, updated_at FROM driver_locations
WHERE driver_id = $1;

-- name: FindNearbyDrivers :many
SELECT d.driver_id, d.location, d.heading, d.updated_at,
       u.name AS driver_name, u.rating,
       sqrt(power((d.location[0] - (sqlc.arg(location)::point)[0])::numeric, 2) +
            power((d.location[1] - (sqlc.arg(location)::point)[1])::numeric, 2)) * 111320 AS distance_meters
FROM driver_locations d
JOIN users u ON u.id = d.driver_id AND u.is_active = true AND u.role = 'driver'
JOIN driver_kyc k ON k.driver_id = d.driver_id AND k.status = 'approved'
WHERE d.updated_at > NOW() - interval '10 minutes'
  AND sqrt(power((d.location[0] - (sqlc.arg(location)::point)[0])::numeric, 2) +
           power((d.location[1] - (sqlc.arg(location)::point)[1])::numeric, 2)) * 111320 < sqlc.arg(radius_meters)::float8
ORDER BY distance_meters ASC
LIMIT $1;

-- name: DeleteStaleDriverLocations :exec
DELETE FROM driver_locations
WHERE updated_at < NOW() - interval '1 hour';
