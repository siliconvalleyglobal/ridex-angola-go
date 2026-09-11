-- name: CreateRideSOSEvent :one
INSERT INTO ride_sos_events (ride_id, triggered_by, location, note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListRideSOSEvents :many
SELECT * FROM ride_sos_events
WHERE ride_id = $1
ORDER BY created_at DESC;

-- name: ResolveRideSOSEvent :one
UPDATE ride_sos_events
SET status = 'resolved', resolved_at = NOW(), resolved_by = $2
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: ListActiveSOSEvents :many
SELECT e.*, r.status AS ride_status, r.driver_id AS ride_driver_id,
       r.rider_id AS ride_rider_id
FROM ride_sos_events e
JOIN rides r ON r.id = e.ride_id
WHERE e.status = 'active'
ORDER BY e.created_at DESC
LIMIT $1;

-- name: CreateRideIncidentReport :one
INSERT INTO ride_incident_reports (ride_id, reported_by, category, description)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListRideIncidentReports :many
SELECT * FROM ride_incident_reports
WHERE ride_id = $1
ORDER BY created_at DESC;
