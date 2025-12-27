-- AgentLink Platform Initial Schema Migration
-- This migration creates all core tables for the platform

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- Users Table (creators, developers, admins)
-- ============================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    user_type VARCHAR(20) NOT NULL CHECK (user_type IN ('creator', 'developer', 'admin')),
    wallet_address VARCHAR(42),
    email_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for email lookups during authentication
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_user_type ON users(user_type);

-- ============================================
-- Creator Profiles Table
-- ============================================
CREATE TABLE creator_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name VARCHAR(100) NOT NULL,
    bio TEXT,
    avatar_url VARCHAR(500),
    verified BOOLEAN DEFAULT FALSE,
    total_earnings DECIMAL(18, 8) DEFAULT 0,
    pending_earnings DECIMAL(18, 8) DEFAULT 0
);

-- ============================================
-- Agents Table
-- ============================================
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    
    -- Configuration (encrypted storage)
    config_encrypted BYTEA NOT NULL,
    config_iv BYTEA NOT NULL,
    
    -- Pricing
    price_per_call DECIMAL(10, 6) NOT NULL,
    
    -- Statistics
    total_calls BIGINT DEFAULT 0,
    total_revenue DECIMAL(18, 8) DEFAULT 0,
    average_rating DECIMAL(3, 2) DEFAULT 0,
    review_count INT DEFAULT 0,

    -- Blockchain
    token_id BIGINT,
    token_tx_hash VARCHAR(66),
    
    -- Version
    version INT DEFAULT 1,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for agents
CREATE INDEX idx_agents_creator ON agents(creator_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_category ON agents(category);
CREATE INDEX idx_agents_created_at ON agents(created_at DESC);

-- ============================================
-- Agent Versions Table (version history)
-- ============================================
CREATE TABLE agent_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    version INT NOT NULL,
    config_encrypted BYTEA NOT NULL,
    config_iv BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(agent_id, version)
);

CREATE INDEX idx_agent_versions_agent ON agent_versions(agent_id);

-- ============================================
-- API Keys Table
-- ============================================
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(8) NOT NULL,  -- For display "ak_xxxx..."
    name VARCHAR(100),
    permissions JSONB DEFAULT '{}',
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);

-- ============================================
-- Quotas Table
-- ============================================
CREATE TABLE quotas (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_quota BIGINT DEFAULT 0,
    used_quota BIGINT DEFAULT 0,
    free_quota BIGINT DEFAULT 100,  -- Initial free quota
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================
-- Call Logs Table (partitioned by month)
-- ============================================
CREATE TABLE call_logs (
    id UUID DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    api_key_id UUID NOT NULL,
    user_id UUID NOT NULL,
    
    -- Request information
    request_id VARCHAR(36),
    input_tokens INT,
    output_tokens INT,
    latency_ms INT,
    status VARCHAR(20) CHECK (status IN ('success', 'error', 'timeout')),
    error_code VARCHAR(50),
    
    -- Billing
    cost_usd DECIMAL(10, 6),
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create initial partitions for call_logs (current month and next 3 months)
CREATE TABLE call_logs_2025_01 PARTITION OF call_logs
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE call_logs_2025_02 PARTITION OF call_logs
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE call_logs_2025_03 PARTITION OF call_logs
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');
CREATE TABLE call_logs_2025_04 PARTITION OF call_logs
    FOR VALUES FROM ('2025-04-01') TO ('2025-05-01');

CREATE INDEX idx_call_logs_agent ON call_logs(agent_id);
CREATE INDEX idx_call_logs_user ON call_logs(user_id);
CREATE INDEX idx_call_logs_created ON call_logs(created_at);
CREATE INDEX idx_call_logs_api_key ON call_logs(api_key_id);

-- ============================================
-- Payments Table
-- ============================================
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_usd DECIMAL(10, 2) NOT NULL,
    quota_purchased BIGINT NOT NULL,
    payment_method VARCHAR(20) NOT NULL CHECK (payment_method IN ('stripe', 'coinbase')),
    payment_id VARCHAR(255),  -- External payment ID
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_payments_user ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created ON payments(created_at DESC);

-- ============================================
-- Settlements Table
-- ============================================
CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(18, 8) NOT NULL,
    platform_fee DECIMAL(18, 8) NOT NULL,
    net_amount DECIMAL(18, 8) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    settled_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_settlements_creator ON settlements(creator_id);
CREATE INDEX idx_settlements_status ON settlements(status);

-- ============================================
-- Reviews Table
-- ============================================
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    content TEXT,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(agent_id, user_id)
);

CREATE INDEX idx_reviews_agent ON reviews(agent_id);
CREATE INDEX idx_reviews_user ON reviews(user_id);
CREATE INDEX idx_reviews_status ON reviews(status);

-- ============================================
-- Webhooks Table
-- ============================================
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url VARCHAR(500) NOT NULL,
    events JSONB NOT NULL,  -- ["quota.low", "call.completed"]
    secret VARCHAR(64) NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_webhooks_user ON webhooks(user_id);
CREATE INDEX idx_webhooks_active ON webhooks(active);

-- ============================================
-- Knowledge Files Table
-- ============================================
CREATE TABLE knowledge_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_size BIGINT NOT NULL,
    s3_key VARCHAR(500) NOT NULL,
    chunk_count INT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'processing' CHECK (status IN ('processing', 'completed', 'failed')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_knowledge_files_agent ON knowledge_files(agent_id);
CREATE INDEX idx_knowledge_files_status ON knowledge_files(status);

-- ============================================
-- Trial Usage Table (tracks free trials per agent per user)
-- ============================================
CREATE TABLE trial_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    used_trials INT DEFAULT 0,
    max_trials INT DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, agent_id)
);

CREATE INDEX idx_trial_usage_user ON trial_usage(user_id);
CREATE INDEX idx_trial_usage_agent ON trial_usage(agent_id);

-- ============================================
-- Updated At Trigger Function
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at triggers
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agents_updated_at
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_quotas_updated_at
    BEFORE UPDATE ON quotas
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trial_usage_updated_at
    BEFORE UPDATE ON trial_usage
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
