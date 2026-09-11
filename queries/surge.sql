-- name: CreateSurgeMultiplier :one
INSERT INTO surge_multipliers (zone_id, multiplier, demand_level, reason, valid_until)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveSurgeMultiplier :one
SELECT * FROM surge_multipliers
WHERE zone_id = $1 AND valid_until > NOW()
ORDER BY created_at DESC
LIMIT 1;

-- name: GetSurgeMultipliersByZone :many
SELECT * FROM surge_multipliers
WHERE zone_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: DeactivateExpiredSurge :exec
DELETE FROM surge_multipliers
WHERE valid_until < NOW();

-- name: ListActiveSurgeMultipliers :many
SELECT * FROM surge_multipliers
WHERE valid_until > NOW()
ORDER BY multiplier DESC;

-- name: CreateDemandHeatmap :one
INSERT INTO demand_heatmaps (zone_id, zone_name, active_riders, available_drivers, busy_drivers, ratio, surge_multiplier, demand_trend)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetLatestHeatmapByZone :one
SELECT * FROM demand_heatmaps
WHERE zone_id = $1
ORDER BY recorded_at DESC
LIMIT 1;

-- name: ListLatestHeatmaps :many
SELECT DISTINCT ON (zone_id) *
FROM demand_heatmaps
ORDER BY zone_id, recorded_at DESC;

-- name: GetHeatmapHistory :many
SELECT * FROM demand_heatmaps
WHERE zone_id = $1 AND recorded_at >= $2
ORDER BY recorded_at DESC
LIMIT $3;

-- name: GetSurgeConfig :one
SELECT * FROM surge_configs
ORDER BY updated_at DESC
LIMIT 1;

-- name: UpdateSurgeConfig :one
UPDATE surge_configs
SET enabled = $1, min_multiplier = $2, max_multiplier = $3, 
    threshold_low = $4, threshold_high = $5, threshold_extreme = $6,
    update_interval_sec = $7, decay_rate = $8,
    updated_at = NOW()
RETURNING *;
