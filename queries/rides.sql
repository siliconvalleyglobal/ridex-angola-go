-- name: CreateRide :one
INSERT INTO rides (rider_id, pickup_point, destination_point, pickup_address, destination_address, status, suggested_fare_cents, currency, promo_code, service_zone_id, requested_pickup_at)
VALUES ($1, $2::point, $3::point, $4, $5, 'requested', $6, $7, $8, $9, $10)
RETURNING *;

-- name: CreateRideIdempotent :one
INSERT INTO rides (
    rider_id, pickup_point, destination_point, pickup_address,
    destination_address, status, suggested_fare_cents, currency, promo_code,
    service_zone_id, requested_pickup_at, business_account_id,
    business_member_id, idempotency_key, idempotency_payload_hash
)
VALUES ($1, $2::point, $3::point, $4, $5, 'requested', $6, $7, $8, $9, $10,
        $11, $12, $13, $14)
ON CONFLICT (rider_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL
DO NOTHING
RETURNING *;

-- name: GetRideByIdempotencyKey :one
SELECT * FROM rides
WHERE rider_id = $1 AND idempotency_key = $2;

-- name: GetRideByID :one
SELECT * FROM rides
WHERE id = $1;

-- name: GetRideByIDWithRider :one
SELECT r.*, u.phone AS rider_phone, u.name AS rider_name
FROM rides r
JOIN users u ON u.id = r.rider_id
WHERE r.id = $1;

-- name: GetRideByIDWithDriver :one
SELECT r.*, u.phone AS driver_phone, u.name AS driver_name, u.rating, u.acceptance_rate
FROM rides r
JOIN users u ON u.id = r.driver_id
WHERE r.id = $1;

-- name: MarkRideMatched :one
UPDATE rides
SET status = 'matched', updated_at = NOW()
WHERE rides.id = $1 AND rides.status = 'requested' AND rides.driver_id IS NULL
  AND (rides.requested_pickup_at IS NULL OR rides.requested_pickup_at <= NOW())
  AND NOT EXISTS (SELECT 1 FROM ride_authorizations ra
                  WHERE ra.ride_id = rides.id AND ra.status <> 'approved')
RETURNING *;

-- name: MarkDriverArriving :one
UPDATE rides
SET status = 'driver_arriving', updated_at = NOW()
WHERE id = $1 AND status = 'matched'
  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
RETURNING *;

-- name: MarkRideInProgress :one
UPDATE rides
SET status = 'in_progress', updated_at = NOW()
WHERE id = $1 AND status = 'driver_arriving'
  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
RETURNING *;

-- name: SetAcceptedFare :one
UPDATE rides
SET accepted_fare_cents = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetDriverOnRide :one
UPDATE rides
SET driver_id = $2, status = 'matched', updated_at = NOW()
WHERE rides.id = $1 AND rides.status = 'requested' AND rides.driver_id IS NULL
  AND (rides.requested_pickup_at IS NULL OR rides.requested_pickup_at <= NOW())
  AND NOT EXISTS (SELECT 1 FROM ride_authorizations ra
                  WHERE ra.ride_id = rides.id AND ra.status <> 'approved')
RETURNING *;

-- name: CompleteRide :one
UPDATE rides
SET status = 'completed', completed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: CancelRide :one
UPDATE rides
SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW(),
    cancellation_reason = $2, cancelled_by = $3
WHERE id = $1 AND status IN ('requested', 'matched', 'driver_arriving', 'in_progress')
RETURNING *;

-- name: CancelRideByRider :one
UPDATE rides
SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW(),
    cancellation_reason = $3, cancelled_by = $4
WHERE id = $1 AND rider_id = $2
  AND status IN ('requested', 'matched', 'driver_arriving')
RETURNING *;

-- name: MarkRideNoShow :one
UPDATE rides
SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW(),
    cancellation_reason = 'rider_no_show', cancelled_by = $2
WHERE id = $1 AND driver_id = $2 AND status = 'driver_arriving'
RETURNING *;

-- name: GetRidesByRider :many
SELECT * FROM rides
WHERE rider_id = $1
ORDER BY created_at DESC;

-- name: GetRidesByDriver :many
SELECT * FROM rides
WHERE driver_id = $1
ORDER BY created_at DESC;

-- name: GetRidesByRiderPage :many
SELECT * FROM rides
WHERE rider_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetRidesByDriverPage :many
SELECT * FROM rides
WHERE driver_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetOpenRides :many
SELECT * FROM rides
WHERE status = 'requested' AND driver_id IS NULL
  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
  AND NOT EXISTS (
      SELECT 1 FROM ride_authorizations ra
      WHERE ra.ride_id = rides.id AND ra.status <> 'approved'
  )
ORDER BY created_at ASC
LIMIT $1;

-- name: GetOpenRidesPage :many
SELECT * FROM rides
WHERE status = 'requested' AND driver_id IS NULL
  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
  AND NOT EXISTS (
      SELECT 1 FROM ride_authorizations ra
      WHERE ra.ride_id = rides.id AND ra.status = 'pending'
  )
ORDER BY created_at ASC, id ASC
LIMIT $1 OFFSET $2;

-- name: ListRidesForAdmin :many
SELECT * FROM rides
WHERE ($1::text = '' OR status = $1::text)
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetRidesNearLocation :many
-- Uses Postgres built-in point distance (units are 3D-cartesian; for geo coords,
-- this is an approximation suitable for small-radius matching).
SELECT id, rider_id, pickup_address, destination_address, status, suggested_fare_cents, currency, created_at,
       sqrt(power((pickup_point[0] - $2::point[0])::numeric, 2) + power((pickup_point[1] - $2::point[1])::numeric, 2)) * 111320 AS distance_meters
FROM rides
WHERE status = 'requested'
  AND driver_id IS NULL
  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
  AND NOT EXISTS (SELECT 1 FROM ride_authorizations ra
                  WHERE ra.ride_id = rides.id AND ra.status = 'pending')
  AND sqrt(power((pickup_point[0] - $2::point[0])::numeric, 2) + power((pickup_point[1] - $2::point[1])::numeric, 2)) * 111320 < $3
ORDER BY distance_meters ASC
LIMIT $1;

-- name: ExpireStaleRequestedRides :many
-- Rides still in 'requested' with no accepted driver after the cutoff are
-- expired in bulk by the background reaper. cancelled_by stays NULL: the
-- system, not a user, closed the ride.
UPDATE rides
SET status = 'cancelled',
    cancellation_reason = 'expired',
    cancelled_at = NOW(),
    updated_at = NOW()
WHERE id IN (
    SELECT id FROM rides
    WHERE rides.status = 'requested'
      AND rides.driver_id IS NULL
      AND rides.created_at < $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, rider_id, suggested_fare_cents, currency;

-- name: ReassignRideToRequested :one
-- Automatic reassignment: when the assigned driver cancels a not-yet-started
-- ride, the ride returns to the open pool so other drivers can bid on it.
UPDATE rides
SET status = 'requested',
    driver_id = NULL,
    accepted_fare_cents = NULL,
    updated_at = NOW()
WHERE id = $1 AND driver_id = $2 AND status IN ('matched', 'driver_arriving')
RETURNING *;
