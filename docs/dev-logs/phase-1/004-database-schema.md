# 开发日志 004: 数据库 Schema 设计

**日期**: 2024-12-27  
**任务**: Task 2.1 - 创建 PostgreSQL Schema (基础表)  
**状态**: ✅ 已完成

## 任务描述

设计并创建 AgentLink 平台的核心数据库表结构。

## 实现内容

### 1. 核心表结构

#### users 表
```sql
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

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_wallet ON users(wallet_address) WHERE wallet_address IS NOT NULL;
```

#### agents 表
```sql
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    price_per_call DECIMAL(10, 6) NOT NULL,
    config_encrypted BYTEA,
    config_iv BYTEA,
    version INTEGER DEFAULT 1,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_agents_creator ON agents(creator_id);
CREATE INDEX idx_agents_status ON agents(status);
```

#### api_keys 表
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(11) NOT NULL,
    name VARCHAR(255),
    permissions JSONB DEFAULT '{}',
    last_used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
```

#### quotas 表
```sql
CREATE TABLE quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) UNIQUE,
    total_quota INTEGER DEFAULT 100,
    used_quota INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_quotas_user ON quotas(user_id);
```

#### call_logs 表
```sql
CREATE TABLE call_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    user_id UUID NOT NULL REFERENCES users(id),
    api_key_id UUID REFERENCES api_keys(id),
    request_tokens INTEGER,
    response_tokens INTEGER,
    total_tokens INTEGER,
    cost DECIMAL(10, 6),
    latency_ms INTEGER,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    correlation_id VARCHAR(36),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_call_logs_agent ON call_logs(agent_id);
CREATE INDEX idx_call_logs_user ON call_logs(user_id);
CREATE INDEX idx_call_logs_created ON call_logs(created_at);
```

### 2. 迁移文件

```
backend/migrations/
├── 000001_init_schema.up.sql    # 创建表
└── 000001_init_schema.down.sql  # 回滚表
```

## 遇到的问题

### 问题 1: UUID 扩展未启用

**描述**: gen_random_uuid() 函数不存在

**错误信息**:
```
ERROR: function gen_random_uuid() does not exist
HINT: No function matches the given name and argument types.
```

**解决方案**:
在迁移脚本开头启用 pgcrypto 扩展

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
```

### 问题 2: 外键约束顺序

**描述**: 创建表时外键引用的表尚未创建

**错误信息**:
```
ERROR: relation "users" does not exist
```

**解决方案**:
调整表创建顺序，确保被引用的表先创建

```sql
-- 正确顺序
1. users
2. agents (references users)
3. api_keys (references users)
4. quotas (references users)
5. call_logs (references agents, users, api_keys)
```

### 问题 3: DECIMAL 精度问题

**描述**: 价格计算时精度丢失

**解决方案**:
使用 DECIMAL(10, 6) 确保足够精度，并在 Go 代码中使用 `shopspring/decimal` 库

```go
import "github.com/shopspring/decimal"

type Agent struct {
    PricePerCall decimal.Decimal `json:"price_per_call"`
}
```

## 验证结果

- [x] 所有表创建成功
- [x] 索引创建正确
- [x] 外键约束生效
- [x] 迁移可正常执行和回滚

## 相关文件

- `backend/migrations/000001_init_schema.up.sql`
- `backend/migrations/000001_init_schema.down.sql`
- `backend/internal/models/*.go`
