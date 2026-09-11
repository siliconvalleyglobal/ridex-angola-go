-- Seed 4 de Fevereiro International Airport (LAD)
INSERT INTO airports (iata, name, city, country, country_code, latitude, longitude, timezone, terminal_count, pickup_zones)
VALUES (
    'LAD',
    '4 de Fevereiro International Airport',
    'Luanda',
    'Angola',
    'AO',
    -8.8426,
    13.2346,
    'Africa/Luanda',
    1,
    ARRAY['airport_terminal', 'arrivals_hall', 'departures_hall', 'parking_level_1', 'parking_level_2', 'meet_greet_area']
)
ON CONFLICT (iata) DO NOTHING;
