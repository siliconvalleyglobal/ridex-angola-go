-- Payment analytics for the admin dashboard. All queries are windowed on a
-- single "since" timestamp so the HTTP layer can clamp the window (days).

-- name: PaymentAnalyticsSummary :one
-- Counts and volumes for one status window. FILTER keeps each metric in one
-- pass; ::bigint casts keep the JSON layer on plain int64s.
SELECT
    COUNT(*)                                                              AS total_charges,
    COUNT(*) FILTER (WHERE status = 'completed')                          AS completed_charges,
    COUNT(*) FILTER (WHERE status = 'failed')                             AS failed_charges,
    COUNT(*) FILTER (WHERE status = 'refunded')                           AS refunded_charges,
    COUNT(*) FILTER (WHERE status IN ('pending', 'processing'))           AS in_flight_charges,
    COALESCE(SUM(amount_cents) FILTER (WHERE status = 'completed'), 0)::bigint AS completed_volume_cents,
    COALESCE(SUM(amount_cents) FILTER (WHERE status = 'refunded'), 0)::bigint  AS refunded_volume_cents
FROM payment_charges
WHERE created_at >= $1;

-- name: PaymentDailyVolume :many
-- Per-day completed volume and refunds for trend charts.
SELECT
    date_trunc('day', created_at)::date                                        AS day,
    COUNT(*)                                                                   AS charges,
    COUNT(*) FILTER (WHERE status = 'completed')                               AS completed_charges,
    COALESCE(SUM(amount_cents) FILTER (WHERE status = 'completed'), 0)::bigint AS completed_volume_cents,
    COALESCE(SUM(amount_cents) FILTER (WHERE status = 'refunded'), 0)::bigint  AS refunded_volume_cents
FROM payment_charges
WHERE created_at >= $1
GROUP BY day
ORDER BY day;

-- name: PaymentProviderBreakdown :many
-- Per-provider totals so a misbehaving provider is visible in one glance.
SELECT
    provider                                                                   AS provider,
    COUNT(*)                                                                   AS charges,
    COUNT(*) FILTER (WHERE status = 'completed')                               AS completed_charges,
    COUNT(*) FILTER (WHERE status = 'failed')                                  AS failed_charges,
    COALESCE(SUM(amount_cents) FILTER (WHERE status = 'completed'), 0)::bigint AS completed_volume_cents
FROM payment_charges
WHERE created_at >= $1
GROUP BY provider
ORDER BY charges DESC;