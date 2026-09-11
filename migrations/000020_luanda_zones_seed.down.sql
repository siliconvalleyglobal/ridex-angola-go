-- Remove Luanda zone seed data
DELETE FROM service_zones
WHERE slug IN (
    'luanda-city-center',
    'talatona',
    'kilamba',
    'viana',
    'benfica',
    'cacuaco',
    'alvalade',
    'maianga',
    'rocha-sanchez',
    'cazenga'
);
