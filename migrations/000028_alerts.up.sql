-- Automated Alerts System
CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    condition VARCHAR(50) NOT NULL,
    threshold DECIMAL(10,2) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warning', 'critical')),
    action VARCHAR(50) NOT NULL,
    recipients TEXT[] NOT NULL DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    cooldown_min INTEGER NOT NULL DEFAULT 15,
    last_triggered TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_rules_active ON alert_rules(active);
CREATE INDEX idx_alert_rules_condition ON alert_rules(condition);

-- Alert Events
CREATE TABLE IF NOT EXISTS alert_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    data JSONB DEFAULT '{}',
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_events_rule ON alert_events(rule_id);
CREATE INDEX idx_alert_events_severity ON alert_events(severity);
CREATE INDEX idx_alert_events_ack ON alert_events(acknowledged, created_at DESC);
CREATE INDEX idx_alert_events_created ON alert_events(created_at DESC);

-- Insert default alert rules
INSERT INTO alert_rules (name, description, condition, threshold, severity, action, recipients, cooldown_min)
VALUES 
    ('High Wait Time', 'Average wait time exceeds threshold', 'greater_than', 300, 'warning', 'notify_admin', ARRAY['admin@ridex.ao'], 15),
    ('Driver Shortage', 'Not enough drivers available', 'less_than', 5, 'critical', 'notify_admin_sms', ARRAY['admin@ridex.ao'], 10),
    ('High Cancellation Rate', 'Cancellation rate exceeds threshold', 'greater_than', 20, 'warning', 'notify_admin', ARRAY['admin@ridex.ao'], 30),
    ('Demand Surge', 'Demand surge active in zone', 'greater_than', 2.0, 'info', 'notify_drivers', ARRAY['drivers'], 5)
ON CONFLICT DO NOTHING;

-- Trigger to update timestamp
CREATE OR REPLACE FUNCTION update_alert_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_alert_timestamp
    BEFORE UPDATE ON alert_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_alert_timestamp();
