DROP TABLE IF EXISTS ride_incident_reports;
DROP TABLE IF EXISTS ride_sos_events;
DROP TABLE IF EXISTS emergency_contacts;

DROP INDEX IF EXISTS idx_verification_codes_active;

ALTER TABLE verification_codes
    DROP COLUMN IF EXISTS verified_at;

UPDATE verification_codes
SET code_hash = ''
WHERE code_hash IS NULL;

ALTER TABLE verification_codes
    ALTER COLUMN code_hash SET NOT NULL;

ALTER TABLE verification_codes
    DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;

DELETE FROM verification_codes
WHERE purpose = 'password_reset';

ALTER TABLE verification_codes
    ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('login', 'registration', 'phone_change'));

ALTER TABLE verification_codes
    RENAME COLUMN code_hash TO code;
