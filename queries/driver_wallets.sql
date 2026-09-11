-- name: GetDriverWallet :one
SELECT * FROM driver_wallets
WHERE driver_id = $1;

-- name: CreateDriverWallet :exec
INSERT INTO driver_wallets (driver_id)
VALUES ($1)
ON CONFLICT (driver_id) DO NOTHING;

-- name: AddDriverBalance :one
UPDATE driver_wallets
SET balance_cents = balance_cents + $2,
    total_earned_cents = total_earned_cents + CASE WHEN $2 > 0 THEN $2 ELSE 0 END,
    updated_at = NOW()
WHERE driver_id = $1
RETURNING *;

-- name: DeductDriverBalance :one
UPDATE driver_wallets
SET balance_cents = balance_cents - $2,
    total_paid_cents = total_paid_cents + CASE WHEN $2 > 0 THEN $2 ELSE 0 END,
    updated_at = NOW()
WHERE driver_id = $1 AND balance_cents >= $2
RETURNING *;

-- name: AddDriverEarning :one
UPDATE driver_wallets
SET balance_cents = balance_cents + $2,
    pending_cents = pending_cents + $2,
    total_earned_cents = total_earned_cents + $2,
    updated_at = NOW()
WHERE driver_id = $1
RETURNING *;

-- name: ClearPendingEarnings :one
UPDATE driver_wallets
SET pending_cents = GREATEST(0, pending_cents - $2),
    updated_at = NOW()
WHERE driver_id = $1
RETURNING *;

-- name: UpdateLastPayout :exec
UPDATE driver_wallets
SET last_payout_at = NOW(),
    updated_at = NOW()
WHERE driver_id = $1;

-- name: CreateWalletTransaction :exec
INSERT INTO wallet_transactions (driver_id, type, amount_cents, balance_after, reference_id, reference_type, description, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListWalletTransactions :many
SELECT * FROM wallet_transactions
WHERE driver_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetWalletTransactionByID :one
SELECT * FROM wallet_transactions
WHERE id = $1;

-- name: GetWalletEarningsSummary :one
SELECT 
    COALESCE(SUM(CASE WHEN type = 'earning' AND created_at >= NOW() - INTERVAL '1 day' THEN amount_cents ELSE 0 END), 0) as today_cents,
    COALESCE(SUM(CASE WHEN type = 'earning' AND created_at >= NOW() - INTERVAL '7 days' THEN amount_cents ELSE 0 END), 0) as week_cents,
    COALESCE(SUM(CASE WHEN type = 'earning' AND created_at >= NOW() - INTERVAL '30 days' THEN amount_cents ELSE 0 END), 0) as month_cents,
    COALESCE(SUM(CASE WHEN type = 'earning' THEN amount_cents ELSE 0 END), 0) as total_cents
FROM wallet_transactions
WHERE driver_id = $1 AND type = 'earning' AND status = 'completed';
