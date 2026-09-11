-- Rollback airports and airport transfers

DROP TABLE IF EXISTS airport_transfers;
DROP TABLE IF EXISTS airports;
DROP TYPE IF EXISTS airport_transfer_status;
