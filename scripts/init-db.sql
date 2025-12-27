-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Create initial schema (will be managed by migrations in production)
-- This is just for development convenience

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE agentlink TO postgres;
