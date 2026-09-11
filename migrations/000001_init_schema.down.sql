-- Rollback: drop everything in reverse dependency order.

DROP TABLE IF EXISTS saft_invoices;
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payment_charges;
DROP TABLE IF EXISTS ride_events;
DROP TABLE IF EXISTS rides;
DROP TABLE IF EXISTS verification_codes;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS config;

-- Extensions are not dropped (they may be needed by other databases).
