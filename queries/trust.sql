-- name: SetRidePaymentMethod :one
UPDATE rides SET payment_method = $2, updated_at = NOW()
WHERE id = $1 AND rider_id = $3 AND status = 'requested'
RETURNING *;

-- name: SetRideTripPIN :one
UPDATE rides SET trip_pin_hash = $2, updated_at = NOW()
WHERE id = $1 AND rider_id = $3
RETURNING *;

-- name: VerifyRideTripPIN :one
UPDATE rides SET trip_pin_verified = true, updated_at = NOW()
WHERE id = $1 AND driver_id = $2 AND trip_pin_verified = false
RETURNING *;

-- name: UpsertDriverKYC :one
INSERT INTO driver_kyc (driver_id, id_document, license_number, vehicle_plate, status)
VALUES ($1, $2, $3, $4, 'pending')
ON CONFLICT (driver_id) DO UPDATE SET
    id_document = EXCLUDED.id_document,
    license_number = EXCLUDED.license_number,
    vehicle_plate = EXCLUDED.vehicle_plate,
    status = 'pending',
    submitted_at = NOW(),
    reviewed_at = NULL,
    review_note = NULL
RETURNING driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note;

-- name: GetDriverKYC :one
SELECT driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note
FROM driver_kyc WHERE driver_id = $1;

-- name: ReviewDriverKYC :one
UPDATE driver_kyc
SET status = $2, reviewed_at = NOW(), review_note = $3
WHERE driver_id = $1
RETURNING driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note;

-- name: ListDriverKYC :many
SELECT driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note
FROM driver_kyc
WHERE status = $1
ORDER BY submitted_at ASC
LIMIT $2;

-- name: ListDriverKYCPage :many
SELECT driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note
FROM driver_kyc
WHERE status = $1
ORDER BY submitted_at ASC, driver_id ASC
LIMIT $2 OFFSET $3;
