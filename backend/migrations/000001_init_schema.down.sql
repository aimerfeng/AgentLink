-- AgentLink Platform Initial Schema Rollback
-- This migration drops all core tables

-- Drop triggers first
DROP TRIGGER IF EXISTS update_trial_usage_updated_at ON trial_usage;
DROP TRIGGER IF EXISTS update_quotas_updated_at ON quotas;
DROP TRIGGER IF EXISTS update_agents_updated_at ON agents;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order of creation (respecting foreign key constraints)
DROP TABLE IF EXISTS trial_usage;
DROP TABLE IF EXISTS knowledge_files;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS settlements;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS call_logs_2025_04;
DROP TABLE IF EXISTS call_logs_2025_03;
DROP TABLE IF EXISTS call_logs_2025_02;
DROP TABLE IF EXISTS call_logs_2025_01;
DROP TABLE IF EXISTS call_logs;
DROP TABLE IF EXISTS quotas;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS agent_versions;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS creator_profiles;
DROP TABLE IF EXISTS users;

-- Drop extensions (optional, usually kept)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
