-- AgentLink Platform pgvector Extension Rollback
-- This migration removes the knowledge_embeddings table and pgvector extension

-- Drop the helper function
DROP FUNCTION IF EXISTS search_knowledge_embeddings(UUID, vector(1536), INT, FLOAT);

-- Drop the knowledge_embeddings table
DROP TABLE IF EXISTS knowledge_embeddings;

-- Note: We don't drop the vector extension as it might be used by other applications
-- If you want to drop it, uncomment the following line:
-- DROP EXTENSION IF EXISTS vector;
