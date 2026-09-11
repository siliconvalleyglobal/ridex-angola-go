DROP INDEX IF EXISTS idx_rides_business_account;
ALTER TABLE rides
    DROP COLUMN IF EXISTS business_member_id,
    DROP COLUMN IF EXISTS business_account_id;
DROP INDEX IF EXISTS idx_ride_authorizations_pending;
DROP INDEX IF EXISTS idx_ride_authorizations_account_status;
DROP TABLE IF EXISTS ride_authorizations;
DROP INDEX IF EXISTS idx_business_members_user;
DROP TABLE IF EXISTS business_members;
DROP INDEX IF EXISTS idx_business_accounts_owner;
DROP TABLE IF EXISTS business_accounts;
