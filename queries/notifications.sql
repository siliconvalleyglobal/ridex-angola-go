-- name: CreateNotification :one
INSERT INTO notifications (user_id, type, title, body, data)
VALUES ($1, $2, $3, $4, COALESCE(sqlc.arg(data)::jsonb, '{}'::jsonb))
RETURNING *;

-- name: ListUnreadNotifications :many
SELECT * FROM notifications
WHERE user_id = $1 AND read_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListReadNotifications :many
SELECT * FROM notifications
WHERE user_id = $1 AND read_at IS NOT NULL
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: MarkNotificationRead :one
UPDATE notifications
SET read_at = COALESCE(read_at, NOW())
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: GetNotificationPreferences :one
SELECT * FROM notification_preferences
WHERE user_id = $1;

-- name: UpsertNotificationPreferences :one
INSERT INTO notification_preferences (
    user_id, ride_updates, payment_updates, kyc_updates, marketing
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
    ride_updates = EXCLUDED.ride_updates,
    payment_updates = EXCLUDED.payment_updates,
    kyc_updates = EXCLUDED.kyc_updates,
    marketing = EXCLUDED.marketing,
    updated_at = NOW()
RETURNING *;

-- name: DeleteNotificationPreferences :exec
DELETE FROM notification_preferences
WHERE user_id = $1;

-- name: UpsertDeviceToken :one
INSERT INTO device_tokens (user_id, token, platform)
VALUES ($1, $2, $3)
ON CONFLICT (token) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    platform = EXCLUDED.platform,
    updated_at = NOW()
RETURNING *;

-- name: ListDeviceTokensByUser :many
SELECT * FROM device_tokens WHERE user_id = $1 ORDER BY updated_at DESC;

-- name: DeleteDeviceToken :exec
DELETE FROM device_tokens WHERE token = $1 AND user_id = $2;

-- name: DeleteInvalidDeviceToken :exec
DELETE FROM device_tokens WHERE token = $1;
