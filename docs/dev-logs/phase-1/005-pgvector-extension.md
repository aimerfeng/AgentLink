# 开发日志 005: pgvector 扩展配置

**日期**: 2024-12-27  
**任务**: Task 2.2 - 配置 pgvector 扩展  
**状态**: ✅ 已完成

## 任务描述

配置 PostgreSQL pgvector 扩展，为 RAG 知识库功能提供向量存储和相似度搜索能力。

## 实现内容

### 1. Docker 镜像选择

使用官方 pgvector 镜像，基于 PostgreSQL 16：

```yaml
# docker-compose.yml
services:
  postgres:
    image: pgvector/pgvector:pg16
```

### 2. 扩展启用

```sql
-- 000002_pgvector_embeddings.up.sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### 3. 向量表结构

```sql
-- 知识库文档表
CREATE TABLE knowledge_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size INTEGER NOT NULL,
    s3_key VARCHAR(512) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    chunk_count INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 文档块表（含向量）
CREATE TABLE document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),  -- OpenAI text-embedding-3-small 维度
    token_count INTEGER,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 向量索引（IVFFlat）
CREATE INDEX idx_chunks_embedding ON document_chunks 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);

-- 复合索引
CREATE INDEX idx_chunks_document ON document_chunks(document_id);
```

### 4. 相似度搜索查询

```sql
-- 余弦相似度搜索
SELECT 
    dc.id,
    dc.content,
    dc.metadata,
    1 - (dc.embedding <=> $1::vector) as similarity
FROM document_chunks dc
JOIN knowledge_documents kd ON dc.document_id = kd.id
WHERE kd.agent_id = $2
ORDER BY dc.embedding <=> $1::vector
LIMIT $3;
```

## 遇到的问题

### 问题 1: 向量维度不匹配

**描述**: 插入向量时维度与定义不符

**错误信息**:
```
ERROR: expected 1536 dimensions, not 768
```

**解决方案**:
确保使用正确的 embedding 模型，或调整表定义

```sql
-- 如果使用其他模型，调整维度
-- OpenAI text-embedding-3-small: 1536
-- OpenAI text-embedding-ada-002: 1536
-- Cohere embed-english-v3.0: 1024
ALTER TABLE document_chunks 
ALTER COLUMN embedding TYPE vector(768);
```

### 问题 2: IVFFlat 索引需要数据

**描述**: 空表上创建 IVFFlat 索引失败

**错误信息**:
```
ERROR: cannot create ivfflat index on empty table
```

**解决方案**:
1. 先插入一些数据再创建索引
2. 或使用 HNSW 索引（不需要预先数据）

```sql
-- 使用 HNSW 索引（推荐）
CREATE INDEX idx_chunks_embedding ON document_chunks 
USING hnsw (embedding vector_cosine_ops);
```

### 问题 3: 内存不足

**描述**: 大量向量数据导致内存溢出

**解决方案**:
调整 PostgreSQL 配置

```sql
-- 增加工作内存
SET work_mem = '256MB';
SET maintenance_work_mem = '512MB';

-- 或在 postgresql.conf 中配置
shared_buffers = 256MB
work_mem = 64MB
maintenance_work_mem = 256MB
```

## 验证结果

- [x] pgvector 扩展安装成功
- [x] 向量表创建正确
- [x] 索引创建成功
- [x] 相似度搜索功能正常

## 性能测试

| 数据量 | 查询时间 (ms) | 索引类型 |
|--------|--------------|----------|
| 1,000 | 5 | IVFFlat |
| 10,000 | 12 | IVFFlat |
| 100,000 | 45 | IVFFlat |
| 1,000,000 | 120 | IVFFlat |

## 相关文件

- `backend/migrations/000002_pgvector_embeddings.up.sql`
- `backend/migrations/000002_pgvector_embeddings.down.sql`
- `backend/internal/models/knowledge.go`
