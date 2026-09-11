-- name: CreateIdentityVerification :one
INSERT INTO identity_verifications (user_id, status, id_document_type, id_document_url, selfie_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIdentityVerificationByUser :one
SELECT * FROM identity_verifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetIdentityVerificationByID :one
SELECT * FROM identity_verifications
WHERE id = $1;

-- name: UpdateIdentityVerificationStatus :one
UPDATE identity_verifications
SET status = $2, verified_at = $3, verified_by = $4, confidence_score = $5, rejection_reason = $6, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ApproveIdentityVerification :one
UPDATE identity_verifications
SET status = 'verified', verified_at = NOW(), verified_by = $2, confidence_score = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RejectIdentityVerification :one
UPDATE identity_verifications
SET status = 'rejected', rejection_reason = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListPendingVerifications :many
SELECT * FROM identity_verifications
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: CreateDriverBadge :one
INSERT INTO driver_badges (driver_id, badge_type, display_name, description, valid_until)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListDriverBadges :many
SELECT * FROM driver_badges
WHERE driver_id = $1 AND active = TRUE
ORDER BY earned_at DESC;

-- name: DeactivateDriverBadge :exec
UPDATE driver_badges
SET active = FALSE
WHERE id = $1;

-- name: ExpireDriverBadges :exec
UPDATE driver_badges
SET active = FALSE
WHERE valid_until < NOW() AND active = TRUE;
