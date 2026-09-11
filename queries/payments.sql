-- name: CreatePaymentCharge :one
INSERT INTO payment_charges (ride_id, provider, provider_charge_id, amount_cents, currency, status, raw_response)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreatePaymentIntent :one
INSERT INTO payment_charges (
    ride_id, provider, provider_charge_id, idempotency_key,
    amount_cents, currency, status, raw_response
)
VALUES ($1, $2, $3, $4, $5, $6, 'pending', '{}'::jsonb)
ON CONFLICT (ride_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL
DO UPDATE SET updated_at = payment_charges.updated_at
RETURNING *;

-- name: GetPaymentChargeByIdempotencyKey :one
SELECT * FROM payment_charges
WHERE ride_id = $1 AND idempotency_key = $2;

-- name: GetPaymentChargeByID :one
SELECT * FROM payment_charges
WHERE id = $1;

-- name: GetPaymentChargeByProviderChargeID :one
SELECT * FROM payment_charges
WHERE provider = $1 AND provider_charge_id = $2;

-- name: UpdatePaymentChargeStatus :one
UPDATE payment_charges
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ApplyPaymentEventStatus :one
UPDATE payment_charges
SET status = $2,
    completed_at = CASE
        WHEN $2 = 'completed' THEN COALESCE(completed_at, NOW())
        ELSE completed_at
    END,
    refunded_at = CASE
        WHEN $2 = 'refunded' THEN COALESCE(refunded_at, NOW())
        ELSE refunded_at
    END,
    updated_at = NOW()
WHERE id = $1
  AND (
      ($2 = 'processing' AND status IN ('pending', 'processing'))
      OR ($2 = 'completed' AND status IN ('pending', 'processing', 'completed'))
      OR ($2 = 'failed' AND status IN ('pending', 'processing', 'failed'))
      OR ($2 = 'refunded' AND status IN ('completed', 'refunded'))
  )
RETURNING *;

-- name: CompletePaymentCharge :one
UPDATE payment_charges
SET status = 'completed', completed_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: FailPaymentCharge :one
UPDATE payment_charges
SET status = 'failed', updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RefundPaymentCharge :one
UPDATE payment_charges
SET status = 'refunded', refunded_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetPaymentChargesByRide :many
SELECT * FROM payment_charges
WHERE ride_id = $1
ORDER BY created_at ASC;

-- name: GetPendingCharges :many
SELECT * FROM payment_charges
WHERE status IN ('pending', 'processing')
ORDER BY created_at ASC
LIMIT $1;

-- name: ListPaymentCharges :many
SELECT * FROM payment_charges
WHERE ($1::text = '' OR status = $1::text)
  AND ($2::text = '' OR provider = $2::text)
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;
