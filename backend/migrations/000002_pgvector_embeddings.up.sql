-- AgentLink Platform pgvector Extension Migration
-- This migration enables pgvector and creates the knowledge_embeddings table

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================
-- Knowledge Embeddings Table
-- Stores vector embeddings for RAG retrieval
-- ============================================
CREATE TABLE knowledge_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES knowledge_files(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),  -- OpenAI text-embedding-3-small dimension
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Standard indexes
CREATE INDEX idx_embeddings_agent ON knowledge_embeddings(agent_id);
CREATE INDEX idx_embeddings_file ON knowledge_embeddings(file_id);

-- Vector similarity search index using IVFFlat
-- IVFFlat is good for approximate nearest neighbor search
-- lists = 100 is suitable for datasets up to ~1M vectors
CREATE INDEX idx_embeddings_vector ON knowledge_embeddings 
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Alternative: HNSW index for better recall (uncomment if preferred)
-- CREATE INDEX idx_embeddings_vector_hnsw ON knowledge_embeddings 
-- USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- ============================================
-- Helper function for similarity search
-- ============================================
CREATE OR REPLACE FUNCTION search_knowledge_embeddings(
    p_agent_id UUID,
    p_query_embedding vector(1536),
    p_limit INT DEFAULT 5,
    p_similarity_threshold FLOAT DEFAULT 0.7
)
RETURNS TABLE (
    id UUID,
    file_id UUID,
    chunk_index INT,
    content TEXT,
    metadata JSONB,
    similarity FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        ke.id,
        ke.file_id,
        ke.chunk_index,
        ke.content,
        ke.metadata,
        1 - (ke.embedding <=> p_query_embedding) AS similarity
    FROM knowledge_embeddings ke
    WHERE ke.agent_id = p_agent_id
      AND 1 - (ke.embedding <=> p_query_embedding) >= p_similarity_threshold
    ORDER BY ke.embedding <=> p_query_embedding
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- Comments for documentation
-- ============================================
COMMENT ON TABLE knowledge_embeddings IS 'Stores vector embeddings for RAG knowledge retrieval';
COMMENT ON COLUMN knowledge_embeddings.embedding IS 'OpenAI text-embedding-3-small 1536-dimensional vector';
COMMENT ON COLUMN knowledge_embeddings.chunk_index IS 'Index of the chunk within the source file';
COMMENT ON COLUMN knowledge_embeddings.metadata IS 'Additional metadata like source page, section headers, etc.';
COMMENT ON FUNCTION search_knowledge_embeddings IS 'Performs cosine similarity search on knowledge embeddings';
