-- Bearer tokens for read-only live ride sharing. Only a SHA-256 digest is
-- persisted; the opaque token is returned once when it is created.
CREATE TABLE ride_share_tokens (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id     UUID        NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    rider_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  BYTEA       NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ride_share_tokens_expiry_after_creation
        CHECK (expires_at > created_at)
);

CREATE INDEX idx_ride_share_tokens_ride
    ON ride_share_tokens (ride_id, created_at DESC);

CREATE INDEX idx_ride_share_tokens_active
    ON ride_share_tokens (token_hash, expires_at)
    WHERE revoked_at IS NULL;
