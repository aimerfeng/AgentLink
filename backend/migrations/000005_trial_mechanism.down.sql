-- Rollback Trial Mechanism Migration

-- Drop index first
DROP INDEX IF EXISTS idx_agents_trial_enabled;

-- Remove trial_enabled column from agents table
ALTER TABLE agents DROP COLUMN IF EXISTS trial_enabled;
