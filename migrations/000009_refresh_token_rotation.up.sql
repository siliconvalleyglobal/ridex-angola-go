ALTER TABLE sessions
    ADD COLUMN refresh_token_hash TEXT,
    ADD COLUMN revoked_at TIMESTAMPTZ;

CREATE UNIQUE INDEX idx_sessions_refresh_token_hash
    ON sessions(refresh_token_hash)
    WHERE refresh_token_hash IS NOT NULL;

CREATE INDEX idx_sessions_active
    ON sessions(user_id, expires_at)
    WHERE revoked_at IS NULL;
