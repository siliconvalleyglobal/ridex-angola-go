-- name: CreatePaymentEvent :one
INSERT INTO payment_events (charge_id, event_type, payload, signature)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RecordPaymentEvent :one
INSERT INTO payment_events (
    charge_id, event_type, payload, signature, provider_event_id
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (charge_id, provider_event_id)
    WHERE provider_event_id IS NOT NULL
DO UPDATE SET charge_id = payment_events.charge_id
RETURNING *;

-- name: GetPaymentEventByProviderEventID :one
SELECT * FROM payment_events
WHERE charge_id = $1 AND provider_event_id = $2;

-- name: GetPaymentEvents :many
SELECT * FROM payment_events
WHERE charge_id = $1
ORDER BY received_at ASC;
