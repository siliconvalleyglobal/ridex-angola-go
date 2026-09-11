DROP INDEX IF EXISTS idx_sessions_active;
DROP INDEX IF EXISTS idx_sessions_refresh_token_hash;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS refresh_token_hash;
