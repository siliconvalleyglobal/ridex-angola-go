-- name: CreateEmergencyContact :one
INSERT INTO emergency_contacts (user_id, name, phone, relationship)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEmergencyContacts :many
SELECT * FROM emergency_contacts
WHERE user_id = $1
ORDER BY created_at ASC, id ASC;

-- name: GetEmergencyContact :one
SELECT * FROM emergency_contacts
WHERE id = $1 AND user_id = $2;

-- name: UpdateEmergencyContact :one
UPDATE emergency_contacts
SET name = $3, phone = $4, relationship = $5, updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteEmergencyContact :one
DELETE FROM emergency_contacts
WHERE id = $1 AND user_id = $2
RETURNING *;
