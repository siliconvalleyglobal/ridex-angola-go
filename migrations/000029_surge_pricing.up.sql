-- Surge Pricing System
CREATE TABLE IF NOT EXISTS surge_multipliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID REFERENCES service_zones(id) ON DELETE CASCADE,
    multiplier DECIMAL(3,2) NOT NULL DEFAULT 1.00 CHECK (multiplier >= 1.00 AND multiplier <= 5.00),
    demand_level VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK (demand_level IN ('low', 'normal', 'high', 'extreme')),
    reason VARCHAR(200),
    valid_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_surge_zone ON surge_multipliers(zone_id, valid_until);
CREATE INDEX idx_surge_demand ON surge_multipliers(demand_level);

-- Demand Heatmap History
CREATE TABLE IF NOT EXISTS demand_heatmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID NOT NULL REFERENCES service_zones(id) ON DELETE CASCADE,
    zone_name VARCHAR(100) NOT NULL,
    active_riders INTEGER NOT NULL DEFAULT 0,
    available_drivers INTEGER NOT NULL DEFAULT 0,
    busy_drivers INTEGER NOT NULL DEFAULT 0,
    ratio DECIMAL(5,2),
    surge_multiplier DECIMAL(3,2) DEFAULT 1.00,
    demand_trend VARCHAR(20) DEFAULT 'stable' CHECK (demand_trend IN ('increasing', 'stable', 'decreasing')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_heatmap_zone ON demand_heatmaps(zone_id, recorded_at DESC);
CREATE INDEX idx_heatmap_recorded ON demand_heatmaps(recorded_at);

-- Surge Configuration
CREATE TABLE IF NOT EXISTS surge_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    min_multiplier DECIMAL(3,2) NOT NULL DEFAULT 1.00,
    max_multiplier DECIMAL(3,2) NOT NULL DEFAULT 3.00,
    threshold_low DECIMAL(5,2) NOT NULL DEFAULT 0.50,
    threshold_high DECIMAL(5,2) NOT NULL DEFAULT 1.50,
    threshold_extreme DECIMAL(5,2) NOT NULL DEFAULT 2.50,
    update_interval_sec INTEGER NOT NULL DEFAULT 60,
    decay_rate DECIMAL(3,2) NOT NULL DEFAULT 0.90,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default surge config
INSERT INTO surge_configs (id, enabled, min_multiplier, max_multiplier, threshold_low, threshold_high, threshold_extreme, update_interval_sec, decay_rate)
VALUES (gen_random_uuid(), TRUE, 1.00, 3.00, 0.50, 1.50, 2.50, 60, 0.90)
ON CONFLICT DO NOTHING;
