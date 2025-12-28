-- Trial Mechanism Migration
-- Implements Requirements D5.1, D5.2, D5.4

-- Add trial_enabled column to agents table
-- D5.4: THE Creator SHALL have option to disable trial for their Agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS trial_enabled BOOLEAN DEFAULT TRUE;

-- Create index for trial_enabled queries
CREATE INDEX IF NOT EXISTS idx_agents_trial_enabled ON agents(trial_enabled);

-- Add comment for documentation
COMMENT ON COLUMN agents.trial_enabled IS 'Whether trial calls are enabled for this agent (D5.4)';
