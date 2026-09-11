-- name: CreateRideShareToken :one
INSERT INTO ride_share_tokens (ride_id, rider_id, token_hash, expires_at)
SELECT $1, $2, $3, $4
FROM rides
WHERE id = $1 AND rider_id = $2
RETURNING id, ride_id, created_at, expires_at, revoked_at;

-- name: ListRideShareTokens :many
SELECT id, ride_id, created_at, expires_at, revoked_at
FROM ride_share_tokens
WHERE ride_id = $1 AND rider_id = $2
ORDER BY created_at DESC, id DESC;

-- name: RevokeRideShareToken :one
UPDATE ride_share_tokens
SET revoked_at = COALESCE(revoked_at, NOW())
WHERE id = $1 AND ride_id = $2 AND rider_id = $3
RETURNING id, ride_id, created_at, expires_at, revoked_at;

-- name: GetRideShareByTokenHash :one
SELECT
    st.id AS share_id,
    st.ride_id,
    st.expires_at,
    r.status,
    r.pickup_address,
    r.pickup_point,
    r.destination_address,
    r.destination_point,
    d.name AS driver_name,
    vp.license_plate AS vehicle_license_plate,
    vp.make AS vehicle_make,
    vp.model AS vehicle_model,
    vp.year AS vehicle_year,
    vp.color AS vehicle_color
FROM ride_share_tokens st
JOIN rides r ON r.id = st.ride_id
LEFT JOIN users d ON d.id = r.driver_id AND d.role = 'driver'
LEFT JOIN driver_vehicle_profiles vp ON vp.driver_id = r.driver_id
WHERE st.token_hash = $1
  AND st.revoked_at IS NULL
  AND st.expires_at > NOW();
