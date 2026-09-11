-- Fraud signal probe for charge creation: one pass over the rider's recent
-- charges. $2 is the outer scan bound (the earlier of the two evaluation
-- windows); $3/$4 are the velocity and duplicate sub-windows evaluated with
-- FILTER inside the scan, so both signals cost a single scan.
-- name: GetRiderChargeSignals :one
SELECT
    COUNT(*) FILTER (WHERE pc.created_at >= $3)                          AS recent_charges,
    COUNT(*) FILTER (WHERE pc.created_at >= $4 AND pc.amount_cents = $5) AS duplicate_amounts
FROM payment_charges pc
JOIN rides r ON r.id = pc.ride_id
WHERE r.rider_id = $1 AND pc.created_at >= $2;