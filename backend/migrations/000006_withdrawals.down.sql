-- Withdrawals Migration Rollback
-- Drops the withdrawals table

DROP INDEX IF EXISTS idx_withdrawals_pending;
DROP INDEX IF EXISTS idx_withdrawals_created;
DROP INDEX IF EXISTS idx_withdrawals_status;
DROP INDEX IF EXISTS idx_withdrawals_creator;
DROP TABLE IF EXISTS withdrawals;
