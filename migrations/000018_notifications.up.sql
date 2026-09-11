-- Locally persisted, in-app notifications. External push/SMS delivery is
-- intentionally outside this schema and service.
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    data        JSONB NOT NULL DEFAULT '{}',
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_unread
    ON notifications (user_id, created_at DESC)
    WHERE read_at IS NULL;
CREATE INDEX idx_notifications_user_read
    ON notifications (user_id, created_at DESC)
    WHERE read_at IS NOT NULL;

CREATE TABLE notification_preferences (
    user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    ride_updates     BOOLEAN NOT NULL DEFAULT true,
    payment_updates  BOOLEAN NOT NULL DEFAULT true,
    kyc_updates      BOOLEAN NOT NULL DEFAULT true,
    marketing        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
