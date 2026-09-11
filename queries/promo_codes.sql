-- name: CreatePromoCode :one
INSERT INTO promo_codes (code, discount_type, discount_cents, max_uses, valid_from, valid_until, min_ride_cents, max_discount_cents, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPromoCodeByCode :one
SELECT * FROM promo_codes
WHERE code = $1;

-- name: GetPromoCodeByID :one
SELECT * FROM promo_codes
WHERE id = $1;

-- name: IncrementPromoUsage :one
UPDATE promo_codes
SET current_uses = current_uses + 1,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeactivatePromoCode :exec
UPDATE promo_codes
SET is_active = FALSE,
    updated_at = NOW()
WHERE id = $1;

-- name: ListActivePromoCodes :many
SELECT * FROM promo_codes
WHERE is_active = TRUE AND valid_from <= NOW() AND valid_until >= NOW()
ORDER BY created_at DESC;

-- name: RecordPromoUsage :exec
INSERT INTO promo_code_usages (promo_id, user_id, ride_id, discount_cents)
VALUES ($1, $2, $3, $4);

-- name: CheckUserPromoUsage :one
SELECT EXISTS(
    SELECT 1 FROM promo_code_usages
    WHERE promo_id = $1 AND user_id = $2
) as used;

-- name: GetUserPromoUsages :many
SELECT * FROM promo_code_usages
WHERE user_id = $1
ORDER BY used_at DESC;

-- name: GetLoyaltyBalance :one
SELECT * FROM loyalty_points
WHERE user_id = $1;

-- name: CreateLoyaltyPoints :exec
INSERT INTO loyalty_points (user_id)
VALUES ($1)
ON CONFLICT (user_id) DO NOTHING;

-- name: AddLoyaltyPoints :one
UPDATE loyalty_points
SET balance = balance + $2,
    total_earned = total_earned + CASE WHEN $2 > 0 THEN $2 ELSE 0 END,
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: DeductLoyaltyPoints :one
UPDATE loyalty_points
SET balance = balance - $2,
    total_redeemed = total_redeemed + CASE WHEN $2 > 0 THEN $2 ELSE 0 END,
    updated_at = NOW()
WHERE user_id = $1 AND balance >= $2
RETURNING *;

-- name: UpdateLoyaltyTier :exec
UPDATE loyalty_points
SET tier = $2,
    updated_at = NOW()
WHERE user_id = $1;

-- name: CreateLoyaltyTransaction :exec
INSERT INTO loyalty_transactions (user_id, points, type, reason, reference_id, reference_type)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListLoyaltyTransactions :many
SELECT * FROM loyalty_transactions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
