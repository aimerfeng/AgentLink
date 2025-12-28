-- AgentLink Platform Level 3 Multi-Agent Orchestration Migration
-- This migration creates all tables for Squad, Workflow, and Execution management

-- ============================================
-- Squads Table (Agent Teams)
-- Requirements: B1 - Squad Management
-- ============================================
CREATE TABLE squads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    price_per_execution DECIMAL(10, 6) NOT NULL DEFAULT 0,
    total_executions BIGINT DEFAULT 0,
    total_revenue DECIMAL(18, 8) DEFAULT 0,
    average_rating DECIMAL(3, 2) DEFAULT 0,
    review_count INT DEFAULT 0,
    version INT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for squads
CREATE INDEX idx_squads_creator ON squads(creator_id);
CREATE INDEX idx_squads_status ON squads(status);
CREATE INDEX idx_squads_category ON squads(category);
CREATE INDEX idx_squads_created_at ON squads(created_at DESC);

-- ============================================
-- Squad Members Table (Agent-Squad Association)
-- Requirements: B1.2, B1.3, B1.6
-- ============================================
CREATE TABLE squad_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    role VARCHAR(50),
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(squad_id, agent_id)
);

CREATE INDEX idx_squad_members_squad ON squad_members(squad_id);
CREATE INDEX idx_squad_members_agent ON squad_members(agent_id);


-- ============================================
-- Workflows Table
-- Requirements: B2 - Workflow Definition
-- ============================================
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    -- Encrypted DAG definition (same encryption as Agent configs)
    definition_encrypted BYTEA NOT NULL,
    definition_iv BYTEA NOT NULL,
    price_per_execution DECIMAL(10, 6),
    total_executions BIGINT DEFAULT 0,
    avg_execution_time_ms INT DEFAULT 0,
    success_rate DECIMAL(5, 2) DEFAULT 0,
    version INT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_workflows_squad ON workflows(squad_id);
CREATE INDEX idx_workflows_status ON workflows(status);
CREATE INDEX idx_workflows_created_at ON workflows(created_at DESC);

-- ============================================
-- Workflow Versions Table (Version History)
-- Requirements: B17 - Workflow Version Management
-- ============================================
CREATE TABLE workflow_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    version INT NOT NULL,
    definition_encrypted BYTEA NOT NULL,
    definition_iv BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(workflow_id, version)
);

CREATE INDEX idx_workflow_versions_workflow ON workflow_versions(workflow_id);


-- ============================================
-- Executions Table
-- Requirements: B4, B8 - Orchestrator Engine & Execution Management
-- ============================================
CREATE TABLE executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    workflow_version INT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    api_key_id UUID NOT NULL REFERENCES api_keys(id),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'paused', 'completed', 'failed', 'timeout', 'cancelled'
    )),
    input_params JSONB,
    output_result JSONB,
    current_node_id VARCHAR(100),
    callback_url VARCHAR(500),
    total_cost DECIMAL(10, 6) DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    timeout_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    error_node_id VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_executions_workflow ON executions(workflow_id);
CREATE INDEX idx_executions_user ON executions(user_id);
CREATE INDEX idx_executions_status ON executions(status);
CREATE INDEX idx_executions_created ON executions(created_at DESC);
CREATE INDEX idx_executions_api_key ON executions(api_key_id);

-- ============================================
-- Execution Steps Table
-- Requirements: B4, B8 - Step-by-step execution tracking
-- ============================================
CREATE TABLE execution_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL,
    node_type VARCHAR(20) NOT NULL CHECK (node_type IN (
        'start', 'end', 'agent', 'condition', 'human_approval'
    )),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'completed', 'failed', 'skipped'
    )),
    agent_id UUID REFERENCES agents(id),
    input_tokens INT,
    output_tokens INT,
    latency_ms INT,
    output JSONB,
    error_message TEXT,
    retry_count INT DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_execution_steps_execution ON execution_steps(execution_id);
CREATE INDEX idx_execution_steps_node ON execution_steps(node_id);
CREATE INDEX idx_execution_steps_status ON execution_steps(status);


-- ============================================
-- Shared Contexts Table
-- Requirements: B5 - Shared Context Management
-- ============================================
CREATE TABLE shared_contexts (
    execution_id UUID PRIMARY KEY REFERENCES executions(id) ON DELETE CASCADE,
    context_data JSONB NOT NULL DEFAULT '{}',
    size_bytes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================
-- A2A Messages Table (Agent-to-Agent Communication Log)
-- Requirements: B6 - A2A Communication Protocol
-- ============================================
CREATE TABLE a2a_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    trace_id VARCHAR(36) NOT NULL,
    step INT NOT NULL,
    sender VARCHAR(100) NOT NULL,
    receiver VARCHAR(100) NOT NULL,
    content_type VARCHAR(20) NOT NULL CHECK (content_type IN ('text', 'json', 'file_ref')),
    content_data JSONB,
    content_summary TEXT,
    status VARCHAR(20) NOT NULL CHECK (status IN ('handoff', 'completed', 'error', 'pending')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_a2a_messages_execution ON a2a_messages(execution_id);
CREATE INDEX idx_a2a_messages_trace ON a2a_messages(trace_id);
CREATE INDEX idx_a2a_messages_created ON a2a_messages(created_at DESC);


-- ============================================
-- Human Approvals Table
-- Requirements: B7 - Human-in-the-Loop
-- ============================================
CREATE TABLE human_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'approved', 'rejected', 'timeout'
    )),
    timeout_hours INT DEFAULT 24,
    notification_sent BOOLEAN DEFAULT FALSE,
    decision_by UUID REFERENCES users(id),
    decision_at TIMESTAMP WITH TIME ZONE,
    decision_comment TEXT,
    approval_token VARCHAR(64) UNIQUE,
    token_expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_human_approvals_execution ON human_approvals(execution_id);
CREATE INDEX idx_human_approvals_status ON human_approvals(status);
CREATE INDEX idx_human_approvals_token ON human_approvals(approval_token);
CREATE INDEX idx_human_approvals_expires ON human_approvals(token_expires_at);

-- ============================================
-- Workflow Templates Table
-- Requirements: B13 - Workflow Template Library
-- ============================================
CREATE TABLE workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    -- Template definition stored as plain JSON (not encrypted, as templates are public)
    definition JSONB NOT NULL,
    is_builtin BOOLEAN DEFAULT FALSE,
    creator_id UUID REFERENCES users(id),
    source_workflow_id UUID REFERENCES workflows(id) ON DELETE SET NULL,
    usage_count INT DEFAULT 0,
    average_rating DECIMAL(3, 2) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_workflow_templates_category ON workflow_templates(category);
CREATE INDEX idx_workflow_templates_builtin ON workflow_templates(is_builtin);
CREATE INDEX idx_workflow_templates_creator ON workflow_templates(creator_id);
CREATE INDEX idx_workflow_templates_usage ON workflow_templates(usage_count DESC);


-- ============================================
-- Squad Reviews Table (extends reviews for Squads)
-- Requirements: B10 - Squad Marketplace
-- ============================================
CREATE TABLE squad_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    content TEXT,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(squad_id, user_id)
);

CREATE INDEX idx_squad_reviews_squad ON squad_reviews(squad_id);
CREATE INDEX idx_squad_reviews_user ON squad_reviews(user_id);
CREATE INDEX idx_squad_reviews_status ON squad_reviews(status);

-- ============================================
-- Squad Access Table (tracks purchased/trial access)
-- Requirements: B18.2 - Squad Access Control
-- ============================================
CREATE TABLE squad_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_type VARCHAR(20) NOT NULL CHECK (access_type IN ('purchased', 'trial')),
    trial_executions_used INT DEFAULT 0,
    trial_executions_max INT DEFAULT 3,
    purchased_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(squad_id, user_id)
);

CREATE INDEX idx_squad_access_squad ON squad_access(squad_id);
CREATE INDEX idx_squad_access_user ON squad_access(user_id);
CREATE INDEX idx_squad_access_type ON squad_access(access_type);

-- ============================================
-- Apply updated_at triggers to new tables
-- ============================================
CREATE TRIGGER update_squads_updated_at
    BEFORE UPDATE ON squads
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflows_updated_at
    BEFORE UPDATE ON workflows
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_executions_updated_at
    BEFORE UPDATE ON executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_shared_contexts_updated_at
    BEFORE UPDATE ON shared_contexts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflow_templates_updated_at
    BEFORE UPDATE ON workflow_templates
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- Comments for documentation
-- ============================================
COMMENT ON TABLE squads IS 'Agent teams containing multiple Agents for collaborative AI workflows';
COMMENT ON TABLE squad_members IS 'Association table linking Agents to Squads';
COMMENT ON TABLE workflows IS 'DAG-based workflow definitions for multi-agent orchestration';
COMMENT ON TABLE workflow_versions IS 'Version history for workflows, enabling rollback';
COMMENT ON TABLE executions IS 'Workflow execution instances with status tracking';
COMMENT ON TABLE execution_steps IS 'Individual step records within a workflow execution';
COMMENT ON TABLE shared_contexts IS 'Shared context data passed between agents during execution';
COMMENT ON TABLE a2a_messages IS 'Agent-to-Agent communication log for debugging and audit';
COMMENT ON TABLE human_approvals IS 'Human-in-the-loop approval records for workflow pauses';
COMMENT ON TABLE workflow_templates IS 'Reusable workflow templates (built-in and user-created)';
COMMENT ON TABLE squad_reviews IS 'User reviews and ratings for Squads';
COMMENT ON TABLE squad_access IS 'Tracks user access to Squads (purchased or trial)';

