-- name: CreateBusinessAccount :one
INSERT INTO business_accounts (owner_id, name, currency, monthly_limit_cents, require_ride_approval)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetBusinessAccount :one
SELECT * FROM business_accounts
WHERE id = $1;

-- name: ListBusinessAccountsForUser :many
SELECT DISTINCT ba.*
FROM business_accounts ba
JOIN business_members bm ON bm.account_id = ba.id
WHERE bm.user_id = $1 AND ba.is_active
ORDER BY ba.created_at DESC, ba.id DESC
LIMIT $2 OFFSET $3;

-- name: UpdateBusinessAccountLimits :one
UPDATE business_accounts
SET monthly_limit_cents = $2,
    require_ride_approval = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateBusinessMember :one
INSERT INTO business_members (account_id, user_id, role, spending_limit_cents, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (account_id, user_id)
DO UPDATE SET role = EXCLUDED.role,
              spending_limit_cents = EXCLUDED.spending_limit_cents,
              status = 'active',
              updated_at = NOW()
RETURNING *;

-- name: GetBusinessMember :one
SELECT * FROM business_members
WHERE account_id = $1 AND user_id = $2;

-- name: ListBusinessMembers :many
SELECT * FROM business_members
WHERE account_id = $1
ORDER BY created_at ASC, user_id ASC
LIMIT $2 OFFSET $3;

-- name: UpdateBusinessMemberLimit :one
UPDATE business_members
SET spending_limit_cents = $3, status = $4, updated_at = NOW()
WHERE account_id = $1 AND user_id = $2
RETURNING *;

-- name: GetBusinessMonthlyUsage :one
SELECT
    COUNT(*)::bigint AS ride_count,
    COALESCE(SUM(COALESCE(r.accepted_fare_cents, r.suggested_fare_cents)), 0)::numeric AS total_cents,
    COALESCE(SUM(CASE WHEN r.status = 'completed'
        THEN COALESCE(r.accepted_fare_cents, r.suggested_fare_cents) ELSE 0 END), 0)::numeric AS completed_cents
FROM rides r
WHERE r.business_account_id = $1
  AND r.status <> 'cancelled'
  AND NOT EXISTS (
      SELECT 1 FROM ride_authorizations ra
      WHERE ra.ride_id = r.id AND ra.status = 'rejected'
  )
  AND r.created_at >= date_trunc('month', $2::date)
  AND r.created_at < date_trunc('month', $2::date) + INTERVAL '1 month';

-- name: CreateRideAuthorization :one
INSERT INTO ride_authorizations (
    ride_id, account_id, member_id, status, requested_amount_cents
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (ride_id)
DO UPDATE SET updated_at = ride_authorizations.updated_at
RETURNING *;

-- name: GetRideAuthorization :one
SELECT * FROM ride_authorizations
WHERE ride_id = $1;

-- name: ListPendingRideAuthorizations :many
SELECT * FROM ride_authorizations
WHERE account_id = $1 AND status = 'pending'
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: ApproveRideAuthorization :one
UPDATE ride_authorizations
SET status = 'approved', authorized_by = $2, decision_note = $3,
    decided_at = NOW(), updated_at = NOW()
WHERE ride_id = $1 AND status = 'pending'
RETURNING *;

-- name: RejectRideAuthorization :one
UPDATE ride_authorizations
SET status = 'rejected', authorized_by = $2, decision_note = $3,
    decided_at = NOW(), updated_at = NOW()
WHERE ride_id = $1 AND status = 'pending'
RETURNING *;
