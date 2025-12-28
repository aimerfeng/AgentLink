-- Payment Failure Handling Migration Rollback
-- Removes failure_reason and failed_at columns from payments table
-- Also drops payment_notifications table

-- Drop payment_notifications table
DROP TABLE IF EXISTS payment_notifications;

-- Remove failure tracking columns from payments table
ALTER TABLE payments 
DROP COLUMN IF EXISTS failure_reason,
DROP COLUMN IF EXISTS failed_at;

-- Drop index for failed payments
DROP INDEX IF EXISTS idx_payments_failed_at;
