-- Seed Luanda metropolitan area service zones
-- These zones cover the main operational areas for RideX Angola

INSERT INTO service_zones (name, slug, min_lat, min_lng, max_lat, max_lng, is_active, base_cents, per_km_cents, per_min_cents)
VALUES 
    -- Luanda City Center - Baixa, Samba, Cidade Alta
    ('Luanda City Center', 'luanda-city-center', -9.4800, 13.1800, -9.4300, 13.2500, true, 500, 50, 15),
    -- Talatona - Business district, high-end area
    ('Talatona', 'talatona', -9.4600, 13.2500, -9.4400, 13.2800, true, 600, 55, 16),
    -- Kilamba - Large residential area (Kilamba Kiaxi)
    ('Kilamba', 'kilamba', -9.4900, 13.2400, -9.4400, 13.3000, true, 500, 48, 14),
    -- Viana - Commercial and residential area
    ('Viana', 'viana', -9.5300, 13.2700, -9.4900, 13.3300, true, 500, 45, 13),
    -- Benfica - Residential area
    ('Benfica', 'benfica', -9.5200, 13.2100, -9.4900, 13.2600, true, 450, 45, 13),
    -- Cacuaco - Outskirts, north of Luanda
    ('Cacuaco', 'cacuaco', -9.5800, 13.2400, -9.5200, 13.3200, true, 550, 50, 15),
    -- Alvalade - Residential/commercial
    ('Alvalade', 'alvalade', -9.4900, 13.1800, -9.4600, 13.2300, true, 500, 48, 14),
    -- Maianga - Central area
    ('Maianga', 'maianga', -9.4700, 13.2000, -9.4400, 13.2500, true, 500, 48, 14),
    -- Rocha Sanchez - Residential
    ('Rocha Sanchez', 'rocha-sanchez', -9.5000, 13.2000, -9.4700, 13.2500, true, 450, 45, 13),
    -- Cazenga - Eastern area
    ('Cazenga', 'cazenga', -9.5200, 13.2800, -9.4700, 13.3500, true, 500, 48, 14)
ON CONFLICT (slug) DO NOTHING;
