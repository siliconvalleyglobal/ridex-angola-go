-- name: CreateServiceZone :one
INSERT INTO service_zones (
    name, slug, min_lat, min_lng, max_lat, max_lng, is_active,
    base_cents, per_km_cents, per_min_cents
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetServiceZone :one
SELECT * FROM service_zones
WHERE id = $1;

-- name: ListServiceZones :many
SELECT * FROM service_zones
WHERE ($1::bool = false OR is_active = true)
ORDER BY name ASC, id ASC;

-- name: UpdateServiceZone :one
UPDATE service_zones
SET name = $2, slug = $3, min_lat = $4, min_lng = $5,
    max_lat = $6, max_lng = $7, is_active = $8,
    base_cents = $9, per_km_cents = $10, per_min_cents = $11,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteServiceZone :one
DELETE FROM service_zones
WHERE id = $1
RETURNING *;

-- name: FindServiceZoneForPoint :one
SELECT * FROM service_zones
WHERE is_active = true
  AND $1::numeric BETWEEN min_lat AND max_lat
  AND $2::numeric BETWEEN min_lng AND max_lng
ORDER BY (max_lat - min_lat) * (max_lng - min_lng) ASC, id ASC
LIMIT 1;

-- name: CountActiveServiceZones :one
SELECT COUNT(*)::bigint FROM service_zones
WHERE is_active = true;

-- name: GetServiceZoneBySlug :one
SELECT * FROM service_zones
WHERE slug = $1;
