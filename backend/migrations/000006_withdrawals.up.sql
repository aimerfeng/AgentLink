-- Withdrawals Migration
-- Creates the withdrawals table for creator withdrawal requests

-- Withdrawal status enum
-- pending: withdrawal request created, awaiting processing
-- processing: withdrawal is being processed
-- completed: withdrawal successfully completed
-- failed: withdrawal failed, funds returned to pending_earnings

CREATE TABLE IF NOT EXISTS withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Amount details
    amount DECIMAL(18, 8) NOT NULL,
    platform_fee DECIMAL(18, 8) NOT NULL DEFAULT 0,
    net_amount DECIMAL(18, 8) NOT NULL,
    
    -- Withdrawal method and destination
    withdrawal_method VARCHAR(20) NOT NULL CHECK (withdrawal_method IN ('stripe', 'crypto', 'bank')),
    destination_address VARCHAR(255),  -- Wallet address for crypto, account ID for Stripe
    
    -- Status tracking
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    failure_reason VARCHAR(255),
    
    -- External references
    external_tx_id VARCHAR(255),  -- Stripe payout ID or blockchain tx hash
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for withdrawals
CREATE INDEX IF NOT EXISTS idx_withdrawals_creator ON withdrawals(creator_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);
CREATE INDEX IF NOT EXISTS idx_withdrawals_created ON withdrawals(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_withdrawals_pending ON withdrawals(status) WHERE status = 'pending';

-- Add minimum withdrawal threshold configuration
-- Default minimum: $10.00 USD
-- Platform fee: 2.5% (configurable)
COMMENT ON TABLE withdrawals IS 'Creator withdrawal requests. Minimum threshold: $10.00, Platform fee: 2.5%';
