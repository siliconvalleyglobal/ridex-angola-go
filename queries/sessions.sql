-- name: CreateSession :one
INSERT INTO sessions (id, user_id, device_id, ip_address, user_agent, expires_at, refresh_token_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1;

-- name: RotateSession :one
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1
  AND refresh_token_hash = $2
  AND revoked_at IS NULL
  AND expires_at > NOW()
RETURNING *;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW()
   OR revoked_at IS NOT NULL;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions
WHERE user_id = $1;
