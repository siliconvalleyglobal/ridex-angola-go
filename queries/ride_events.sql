-- name: CreateRideEvent :one
INSERT INTO ride_events (ride_id, event_type, payload, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRideEvents :many
SELECT * FROM ride_events
WHERE ride_id = $1
ORDER BY created_at ASC;

-- name: GetRideEventsPage :many
SELECT * FROM ride_events
WHERE ride_id = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: GetRideEventsByType :many
SELECT * FROM ride_events
WHERE ride_id = $1 AND event_type = $2
ORDER BY created_at ASC;
