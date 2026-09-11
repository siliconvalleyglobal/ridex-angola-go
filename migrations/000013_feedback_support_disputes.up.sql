-- Post-ride feedback, participant support tickets, and ride disputes.

CREATE TABLE ride_reviews (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id     UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL REFERENCES users(id),
    reviewee_id UUID NOT NULL REFERENCES users(id),
    rating      SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     TEXT NOT NULL DEFAULT '' CHECK (char_length(comment) <= 1000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ride_reviews_reviewer_not_reviewee CHECK (reviewer_id <> reviewee_id),
    CONSTRAINT ride_reviews_one_per_reviewer UNIQUE (ride_id, reviewer_id)
);

CREATE INDEX idx_ride_reviews_ride_id ON ride_reviews(ride_id, created_at DESC);
CREATE INDEX idx_ride_reviews_reviewee_id ON ride_reviews(reviewee_id, created_at DESC);

CREATE TABLE support_tickets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    created_by      UUID NOT NULL REFERENCES users(id),
    category        TEXT NOT NULL CHECK (category IN (
        'general', 'payment', 'safety', 'driver', 'rider', 'app_issue', 'other'
    )),
    subject         TEXT NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    description     TEXT NOT NULL CHECK (char_length(description) BETWEEN 1 AND 4000),
    status          TEXT NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
    resolution_note TEXT CHECK (resolution_note IS NULL OR char_length(resolution_note) <= 2000),
    updated_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_support_tickets_ride_id ON support_tickets(ride_id, created_at DESC);
CREATE INDEX idx_support_tickets_created_by ON support_tickets(created_by, created_at DESC);
CREATE INDEX idx_support_tickets_status ON support_tickets(status, updated_at DESC);

CREATE TABLE ride_disputes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id         UUID NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
    filed_by        UUID NOT NULL REFERENCES users(id),
    category        TEXT NOT NULL CHECK (category IN (
        'fare', 'payment', 'service', 'safety', 'other'
    )),
    description     TEXT NOT NULL CHECK (char_length(description) BETWEEN 1 AND 4000),
    status          TEXT NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'in_review', 'resolved', 'rejected')),
    resolution_note TEXT CHECK (resolution_note IS NULL OR char_length(resolution_note) <= 2000),
    resolved_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_ride_disputes_ride_id ON ride_disputes(ride_id, created_at DESC);
CREATE INDEX idx_ride_disputes_filed_by ON ride_disputes(filed_by, created_at DESC);
CREATE INDEX idx_ride_disputes_status ON ride_disputes(status, updated_at DESC);
