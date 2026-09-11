-- name: CreateTripShare :one
INSERT INTO trip_shares (ride_id, rider_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTripShareByRide :one
SELECT * FROM trip_shares
WHERE ride_id = $1;

-- name: GetTripShareByToken :one
SELECT * FROM trip_shares
WHERE token_hash = $1 AND active = TRUE AND expires_at > NOW();

-- name: DeactivateTripShare :exec
UPDATE trip_shares
SET active = FALSE, deactivated_at = NOW()
WHERE ride_id = $1;

-- name: DeactivateExpiredTripShares :exec
UPDATE trip_shares
SET active = FALSE, deactivated_at = NOW()
WHERE active = TRUE AND expires_at < NOW();

-- name: ListActiveTripShares :many
SELECT * FROM trip_shares
WHERE rider_id = $1 AND active = TRUE
ORDER BY created_at DESC;

-- name: AddTripShareViewer :exec
INSERT INTO trip_share_viewers (trip_share_id, contact_id, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (trip_share_id, contact_id) DO NOTHING;

-- name: ListTripShareViewers :many
SELECT * FROM trip_share_viewers
WHERE trip_share_id = $1;
