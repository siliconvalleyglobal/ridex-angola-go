-- name: CreateAlertRule :one
INSERT INTO alert_rules (name, description, condition, threshold, severity, action, recipients, cooldown_min)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAlertRuleByID :one
SELECT * FROM alert_rules
WHERE id = $1;

-- name: ListAlertRules :many
SELECT * FROM alert_rules
ORDER BY created_at DESC;

-- name: ListActiveAlertRules :many
SELECT * FROM alert_rules
WHERE active = TRUE
ORDER BY created_at DESC;

-- name: UpdateAlertRule :one
UPDATE alert_rules
SET name = $2, description = $3, condition = $4, threshold = $5, severity = $6, action = $7, recipients = $8, cooldown_min = $9, active = $10, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateAlertRuleLastTriggered :exec
UPDATE alert_rules
SET last_triggered = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: DeleteAlertRule :exec
DELETE FROM alert_rules
WHERE id = $1;

-- name: CreateAlertEvent :one
INSERT INTO alert_events (rule_id, severity, title, message, data)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAlertEventByID :one
SELECT * FROM alert_events
WHERE id = $1;

-- name: ListAlertEvents :many
SELECT * FROM alert_events
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListUnacknowledgedAlerts :many
SELECT * FROM alert_events
WHERE acknowledged = FALSE
ORDER BY created_at DESC
LIMIT $1;

-- name: ListAlertsBySeverity :many
SELECT * FROM alert_events
WHERE severity = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: AcknowledgeAlertEvent :one
UPDATE alert_events
SET acknowledged = TRUE, acknowledged_by = $2, acknowledged_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteOldAlertEvents :exec
DELETE FROM alert_events
WHERE created_at < NOW() - INTERVAL '30 days';
