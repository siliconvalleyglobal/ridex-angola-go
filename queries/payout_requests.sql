-- Payout request lifecycle queries used by the payouts package. Status
-- transitions are guarded in SQL so concurrent admin actions cannot double
-- process a withdrawal.

-- name: CreatePayoutRequest :one
INSERT INTO payout_requests (driver_id, amount_cents, method, status)
VALUES ($1, $2, $3, 'pending')
RETURNING *;

-- name: GetPayoutRequestByID :one
SELECT * FROM payout_requests
WHERE id = $1;

-- name: GetActivePayoutRequestsByDriver :many
SELECT * FROM payout_requests
WHERE driver_id = $1 AND status IN ('pending', 'processing')
ORDER BY requested_at DESC;

-- name: ListPayoutRequestsByDriver :many
SELECT * FROM payout_requests
WHERE driver_id = $1
ORDER BY requested_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPayoutRequestsByStatus :many
SELECT * FROM payout_requests
WHERE ($1::text = '' OR status = $1::text)
ORDER BY requested_at ASC
LIMIT $2 OFFSET $3;

-- name: MarkPayoutRequestProcessing :one
UPDATE payout_requests
SET status = 'processing'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: CompletePayoutRequest :one
UPDATE payout_requests
SET status = 'completed', processed_at = NOW()
WHERE id = $1 AND status = 'processing'
RETURNING *;

-- name: FailPayoutRequest :one
UPDATE payout_requests
SET status = 'failed', processed_at = NOW(), failure_reason = $2
WHERE id = $1 AND status IN ('pending', 'processing')
RETURNING *;

-- name: SetWalletTransactionStatus :exec
UPDATE wallet_transactions
SET status = $2
WHERE reference_id = $1 AND reference_type = 'payout' AND status = $3;

-- name: GetWalletTransactionByReference :one
SELECT * FROM wallet_transactions
WHERE reference_id = $1 AND reference_type = 'payout'
ORDER BY created_at DESC
LIMIT 1;
