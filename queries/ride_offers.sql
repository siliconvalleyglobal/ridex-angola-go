-- name: AcceptRideOffer :one
WITH accepted AS (
    UPDATE ride_offers AS ro
    SET status = 'accepted', responded_at = NOW()
    WHERE ro.ride_id = $1 AND ro.driver_id = $2 AND ro.status = 'offered' AND ro.expires_at > NOW()
    RETURNING ro.ride_id, ro.driver_id
)
UPDATE rides r
SET driver_id = accepted.driver_id, status = 'matched', updated_at = NOW()
FROM accepted
WHERE r.id = accepted.ride_id AND r.status = 'requested' AND r.driver_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM ride_authorizations ra
                  WHERE ra.ride_id = r.id AND ra.status <> 'approved')
  AND (r.requested_pickup_at IS NULL OR r.requested_pickup_at <= NOW())
RETURNING r.*;

-- name: DeclineRideOffer :exec
UPDATE ride_offers
SET status = 'declined', responded_at = NOW()
WHERE ride_id = $1 AND driver_id = $2 AND status = 'offered';

-- name: GetDriverOffers :many
SELECT id, ride_id, driver_id, status, expires_at, created_at, responded_at
FROM ride_offers
WHERE driver_id = $1 AND status = 'offered' AND expires_at > NOW()
ORDER BY created_at ASC;

-- name: GetDriverOffersPage :many
SELECT id, ride_id, driver_id, status, expires_at, created_at, responded_at
FROM ride_offers
WHERE driver_id = $1 AND status = 'offered' AND expires_at > NOW()
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;
