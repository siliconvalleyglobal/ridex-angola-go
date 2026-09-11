-- name: CreateDriverVehicleProfile :one
INSERT INTO driver_vehicle_profiles (
    driver_id,
    license_plate,
    make,
    model,
    year,
    color,
    license_expiry_date,
    insurance_expiry_date,
    registration_expiry_date,
    inspection_expiry_date
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateDriverVehicleProfile :one
UPDATE driver_vehicle_profiles
SET license_plate = $2,
    make = $3,
    model = $4,
    year = $5,
    color = $6,
    license_expiry_date = $7,
    insurance_expiry_date = $8,
    registration_expiry_date = $9,
    inspection_expiry_date = $10,
    updated_at = NOW()
WHERE driver_id = $1
RETURNING *;

-- name: GetDriverVehicleProfile :one
SELECT *
FROM driver_vehicle_profiles
WHERE driver_id = $1;
