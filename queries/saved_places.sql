-- name: CreateSavedPlace :one
INSERT INTO saved_places (user_id, name, address, latitude, longitude, icon)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListSavedPlaces :many
SELECT * FROM saved_places
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetSavedPlaceByID :one
SELECT * FROM saved_places
WHERE id = $1 AND user_id = $2;

-- name: UpdateSavedPlace :one
UPDATE saved_places
SET name = $3, address = $4, latitude = $5, longitude = $6, icon = $7
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteSavedPlace :exec
DELETE FROM saved_places
WHERE id = $1 AND user_id = $2;

-- name: GetSavedPlaceByIcon :many
SELECT * FROM saved_places
WHERE user_id = $1 AND icon = $2
ORDER BY created_at DESC;
