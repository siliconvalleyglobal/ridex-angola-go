-- name: CreateVerificationCode :one
INSERT INTO verification_codes (user_id, code_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: InvalidateVerificationCodes :exec
UPDATE verification_codes
SET verified_at = COALESCE(verified_at, NOW()),
    expires_at = LEAST(expires_at, NOW())
WHERE user_id = $1
  AND purpose = $2
  AND verified_at IS NULL
  AND expires_at > NOW();

-- name: GetLatestVerificationCode :one
SELECT * FROM verification_codes
WHERE user_id = $1 AND purpose = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetVerificationCode :one
SELECT * FROM verification_codes
WHERE id = $1
  AND expires_at > NOW()
  AND verified_at IS NULL
  AND attempts < max_attempts;

-- name: GetVerifiedVerificationCode :one
SELECT * FROM verification_codes
WHERE id = $1
  AND purpose = 'password_reset'
  AND verified_at IS NOT NULL
  AND expires_at > NOW();

-- name: RecordVerificationAttempt :one
UPDATE verification_codes
SET attempts = attempts + 1
WHERE id = $1
  AND expires_at > NOW()
  AND verified_at IS NULL
  AND attempts < max_attempts
RETURNING *;

-- name: MarkVerificationCodeVerified :one
UPDATE verification_codes
SET verified_at = NOW()
WHERE id = $1
  AND expires_at > NOW()
  AND verified_at IS NULL
  AND attempts <= max_attempts
RETURNING *;

-- name: ConsumeVerifiedVerificationCode :exec
UPDATE verification_codes
SET expires_at = NOW()
WHERE id = $1
  AND verified_at IS NOT NULL;

-- name: DeleteExpiredCodes :exec
DELETE FROM verification_codes
WHERE expires_at < NOW() OR verified_at IS NOT NULL;
