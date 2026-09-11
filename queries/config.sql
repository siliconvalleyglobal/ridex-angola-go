-- name: GetConfig :one
SELECT * FROM config
WHERE key = $1;

-- name: GetAllConfig :many
SELECT * FROM config
ORDER BY key;
