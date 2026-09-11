-- Driver-owned vehicle details and document expiry dates.
CREATE TABLE driver_vehicle_profiles (
    driver_id                 UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    license_plate             TEXT NOT NULL UNIQUE,
    make                      TEXT NOT NULL,
    model                     TEXT NOT NULL,
    year                      INTEGER NOT NULL CHECK (year BETWEEN 1950 AND 2100),
    color                     TEXT NOT NULL,
    license_expiry_date       DATE NOT NULL,
    insurance_expiry_date     DATE NOT NULL,
    registration_expiry_date  DATE,
    inspection_expiry_date    DATE,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_driver_vehicle_profiles_license_expiry
    ON driver_vehicle_profiles (license_expiry_date);

CREATE INDEX idx_driver_vehicle_profiles_insurance_expiry
    ON driver_vehicle_profiles (insurance_expiry_date);
