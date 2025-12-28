-- AgentLink Platform Level 3 Multi-Agent Orchestration Migration Rollback
-- This migration drops all Level 3 tables in reverse dependency order

-- Drop triggers first
DROP TRIGGER IF EXISTS update_workflow_templates_updated_at ON workflow_templates;
DROP TRIGGER IF EXISTS update_shared_contexts_updated_at ON shared_contexts;
DROP TRIGGER IF EXISTS update_executions_updated_at ON executions;
DROP TRIGGER IF EXISTS update_workflows_updated_at ON workflows;
DROP TRIGGER IF EXISTS update_squads_updated_at ON squads;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS squad_access;
DROP TABLE IF EXISTS squad_reviews;
DROP TABLE IF EXISTS workflow_templates;
DROP TABLE IF EXISTS human_approvals;
DROP TABLE IF EXISTS a2a_messages;
DROP TABLE IF EXISTS shared_contexts;
DROP TABLE IF EXISTS execution_steps;
DROP TABLE IF EXISTS executions;
DROP TABLE IF EXISTS workflow_versions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS squad_members;
DROP TABLE IF EXISTS squads;

