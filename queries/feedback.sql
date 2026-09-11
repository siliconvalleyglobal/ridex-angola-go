-- name: CreateRideReview :one
INSERT INTO ride_reviews (ride_id, reviewer_id, reviewee_id, rating, comment)
SELECT $1, $2, $3, $4, $5
FROM rides
WHERE id = $1
  AND status = 'completed'
  AND (
      (rider_id = $2 AND driver_id = $3)
      OR (driver_id = $2 AND rider_id = $3)
  )
ON CONFLICT (ride_id, reviewer_id) DO NOTHING
RETURNING *;

-- name: GetRideReviews :many
SELECT * FROM ride_reviews
WHERE ride_id = $1
ORDER BY created_at ASC, id ASC;

-- name: RefreshUserRating :one
UPDATE users
SET rating = COALESCE((
    SELECT ROUND(AVG(rating)::numeric, 2)
    FROM ride_reviews
    WHERE reviewee_id = $1
), 0),
updated_at = NOW()
WHERE id = $1
RETURNING rating;

-- name: CreateSupportTicket :one
INSERT INTO support_tickets (ride_id, created_by, category, subject, description)
SELECT $1, $2, $3, $4, $5
FROM rides
WHERE id = $1
  AND (rider_id = $2 OR driver_id = $2)
RETURNING *;

-- name: GetSupportTicket :one
SELECT * FROM support_tickets
WHERE id = $1;

-- name: ListSupportTicketsByUser :many
SELECT * FROM support_tickets
WHERE created_by = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListSupportTicketsForRide :many
SELECT * FROM support_tickets
WHERE ride_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListSupportTicketsAdmin :many
SELECT * FROM support_tickets
WHERE ($1::text = '' OR status = $1::text)
ORDER BY updated_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: UpdateSupportTicketStatus :one
UPDATE support_tickets
SET status = $2,
    resolution_note = $3,
    updated_by = $4,
    updated_at = NOW(),
    resolved_at = CASE
        WHEN $2 IN ('resolved', 'closed') THEN COALESCE(resolved_at, NOW())
        ELSE NULL
    END
WHERE id = $1
RETURNING *;

-- name: CreateRideDispute :one
INSERT INTO ride_disputes (ride_id, filed_by, category, description)
SELECT $1, $2, $3, $4
FROM rides
WHERE id = $1
  AND (rider_id = $2 OR driver_id = $2)
RETURNING *;

-- name: GetRideDispute :one
SELECT * FROM ride_disputes
WHERE id = $1;

-- name: ListRideDisputes :many
SELECT * FROM ride_disputes
WHERE ride_id = $1
ORDER BY created_at DESC, id DESC;

-- name: ListRideDisputesAdmin :many
SELECT * FROM ride_disputes
WHERE ($1::text = '' OR status = $1::text)
ORDER BY updated_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: UpdateRideDisputeStatus :one
UPDATE ride_disputes
SET status = $2,
    resolution_note = $3,
    resolved_by = CASE
        WHEN $2 IN ('resolved', 'rejected') THEN $4
        ELSE NULL
    END,
    updated_at = NOW(),
    resolved_at = CASE
        WHEN $2 IN ('resolved', 'rejected') THEN COALESCE(resolved_at, NOW())
        ELSE NULL
    END
WHERE id = $1
RETURNING *;

-- name: GetDriverEarningsSummary :one
WITH completed_rides AS (
    SELECT
        r.id,
        COALESCE(r.accepted_fare_cents, r.suggested_fare_cents) AS gross_fare
    FROM rides r
    WHERE r.driver_id = $1
      AND r.status = 'completed'
),
completed_charges AS (
    SELECT DISTINCT ON (pc.ride_id)
        pc.ride_id,
        pc.amount_cents
    FROM payment_charges pc
    JOIN completed_rides cr ON cr.id = pc.ride_id
    WHERE pc.status = 'completed'
    ORDER BY pc.ride_id, pc.completed_at DESC NULLS LAST, pc.created_at DESC, pc.id DESC
)
SELECT
    COUNT(*)::bigint AS completed_rides,
    COALESCE(SUM(cr.gross_fare), 0)::numeric AS gross_fares,
    COALESCE(SUM(CASE WHEN cp.ride_id IS NOT NULL THEN cp.amount_due ELSE 0 END), 0)::numeric AS cash_earnings,
    COALESCE(SUM(CASE WHEN cc.ride_id IS NOT NULL THEN cc.amount_cents ELSE 0 END), 0)::numeric AS digital_earnings,
    COALESCE(SUM(CASE
        WHEN cp.ride_id IS NOT NULL THEN cp.amount_due
        WHEN cc.ride_id IS NOT NULL THEN cc.amount_cents
        ELSE 0
    END), 0)::numeric AS total_earnings,
    COALESCE(SUM(CASE
        WHEN cp.ride_id IS NULL AND cc.ride_id IS NULL THEN cr.gross_fare
        ELSE 0
    END), 0)::numeric AS unpaid_fares
FROM completed_rides cr
LEFT JOIN cash_payments cp ON cp.ride_id = cr.id
LEFT JOIN completed_charges cc ON cc.ride_id = cr.id;
