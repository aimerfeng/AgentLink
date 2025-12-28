-- Payment Failure Handling Migration
-- Adds failure_reason and failed_at columns to payments table
-- Also creates payment_notifications table for webhook delivery

-- Add failure tracking columns to payments table
ALTER TABLE payments 
ADD COLUMN IF NOT EXISTS failure_reason VARCHAR(255),
ADD COLUMN IF NOT EXISTS failed_at TIMESTAMP WITH TIME ZONE;

-- Create index for failed payments lookup
CREATE INDEX IF NOT EXISTS idx_payments_failed_at ON payments(failed_at) WHERE status = 'failed';

-- Create payment_notifications table for storing notifications to be delivered via webhook
CREATE TABLE IF NOT EXISTS payment_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    delivered BOOLEAN DEFAULT FALSE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    retry_count INT DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for payment_notifications
CREATE INDEX IF NOT EXISTS idx_payment_notifications_user ON payment_notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_notifications_payment ON payment_notifications(payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_notifications_delivered ON payment_notifications(delivered) WHERE delivered = FALSE;
CREATE INDEX IF NOT EXISTS idx_payment_notifications_event_type ON payment_notifications(event_type);
