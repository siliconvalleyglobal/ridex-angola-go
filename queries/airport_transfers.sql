-- Airport transfer queries used by the airport package. Kept in sqlc
-- -- name: format so the generated db layer stays synchronized.

-- name: ListAirports :many
SELECT * FROM airports
ORDER BY name ASC, id ASC;

-- name: GetAirport :one
SELECT * FROM airports
WHERE id = $1;

-- name: GetAirportByIATA :one
SELECT * FROM airports
WHERE iata = $1;

-- name: ListAirportTransfers :many
SELECT * FROM airport_transfers
ORDER BY created_at DESC, id DESC;

-- name: GetAirportTransfer :one
SELECT * FROM airport_transfers
WHERE id = $1;

-- name: GetAirportTransferByRideID :one
SELECT * FROM airport_transfers
WHERE ride_id = $1
LIMIT 1;

-- name: CreateAirportTransfer :one
INSERT INTO airport_transfers (
    ride_id, airport_id, is_pickup, flight_number, airline,
    scheduled_arrival, actual_arrival, passenger_name, passenger_phone,
    meet_and_greet, waiting_minutes, waiting_fee_cents,
    fixed_fare_cents, notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9,
        $10, $11, $12, $13, $14)
RETURNING *;

-- name: UpdateAirportTransfer :one
UPDATE airport_transfers
SET status = $2,
    actual_arrival = $3,
    waiting_minutes = $4,
    notes = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CancelAirportTransfer :one
UPDATE airport_transfers
SET status = 'cancelled',
    notes = COALESCE(NULLIF(sqlc.arg('reason')::text, ''), notes),
    updated_at = NOW()
WHERE id = $1 AND status NOT IN ('completed', 'cancelled', 'no_show')
RETURNING *;
