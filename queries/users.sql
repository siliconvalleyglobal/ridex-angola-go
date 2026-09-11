-- name: CreateUser :one
INSERT INTO users (phone, name, role, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone = $1 AND is_active = true;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND is_active = true;

-- name: UpdateUser :one
UPDATE users
SET name = $2, rating = $3, acceptance_rate = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: DeactivateUser :one
UPDATE users
SET is_active = false, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetUserActive :one
UPDATE users
SET is_active = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users
WHERE ($1::text = '' OR phone ILIKE '%' || $1::text || '%' OR name ILIKE '%' || $1::text || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountUsersByRole :many
SELECT role, COUNT(*) AS count
FROM users
WHERE is_active = true
GROUP BY role;
