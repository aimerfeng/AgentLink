# 开发日志 017: Level 3 数据库迁移

**日期**: 2024-12-28  
**任务**: Task 17 - 创建 Level 3 数据库迁移  
**状态**: ✅ 已完成

## 任务描述

扩展数据库以支持 Level 3 多智能体编排功能，包括 Squad、Workflow、Execution 等表。

## 实现内容

### 1. Squads 和 Squad Members 表

```sql
-- squads 表
CREATE TABLE squads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    price_per_execution DECIMAL(10, 6),
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- squad_members 关联表
CREATE TABLE squad_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id),
    role VARCHAR(50) DEFAULT 'member',
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(squad_id, agent_id)
);

CREATE INDEX idx_squads_creator ON squads(creator_id);
CREATE INDEX idx_squads_status ON squads(status);
CREATE INDEX idx_squad_members_squad ON squad_members(squad_id);
CREATE INDEX idx_squad_members_agent ON squad_members(agent_id);
```

### 2. Workflows 和 Workflow Versions 表

```sql
-- workflows 表
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id UUID NOT NULL REFERENCES squads(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    definition_encrypted BYTEA,
    definition_iv BYTEA,
    version INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- workflow_versions 版本历史表
CREATE TABLE workflow_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    definition_encrypted BYTEA,
    definition_iv BYTEA,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(workflow_id, version)
);

CREATE INDEX idx_workflows_squad ON workflows(squad_id);
CREATE INDEX idx_workflow_versions_workflow ON workflow_versions(workflow_id);
```

### 3. Executions 和 Execution Steps 表

```sql
-- executions 表
CREATE TABLE executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    input JSONB,
    output JSONB,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- execution_steps 步骤记录表
CREATE TABLE execution_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL,
    node_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    input JSONB,
    output JSONB,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_executions_workflow ON executions(workflow_id);
CREATE INDEX idx_executions_user ON executions(user_id);
CREATE INDEX idx_executions_status ON executions(status);
CREATE INDEX idx_execution_steps_execution ON execution_steps(execution_id);
```

### 4. Shared Contexts 和 A2A Messages 表

```sql
-- shared_contexts 持久化表
CREATE TABLE shared_contexts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    namespace VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(execution_id, namespace)
);

-- a2a_messages 日志表
CREATE TABLE a2a_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    from_node VARCHAR(100) NOT NULL,
    to_node VARCHAR(100) NOT NULL,
    message_type VARCHAR(50) NOT NULL,
    content JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_shared_contexts_execution ON shared_contexts(execution_id);
CREATE INDEX idx_a2a_messages_execution ON a2a_messages(execution_id);
```

### 5. Human Approvals 和 Workflow Templates 表

```sql
-- human_approvals 审批记录表
CREATE TABLE human_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    step_id UUID NOT NULL REFERENCES execution_steps(id),
    token VARCHAR(64) UNIQUE NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    approver_email VARCHAR(255),
    decision_data JSONB,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    decided_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- workflow_templates 模板库表
CREATE TABLE workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    definition JSONB NOT NULL,
    is_builtin BOOLEAN DEFAULT FALSE,
    creator_id UUID REFERENCES users(id),
    usage_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_human_approvals_execution ON human_approvals(execution_id);
CREATE INDEX idx_human_approvals_token ON human_approvals(token);
CREATE INDEX idx_workflow_templates_category ON workflow_templates(category);
```

## 遇到的问题

### 问题 1: 外键约束顺序

**描述**: 创建表时外键引用的表尚未创建

**解决方案**:
按依赖顺序创建表

```sql
-- 正确顺序
1. squads
2. squad_members
3. workflows
4. workflow_versions
5. executions
6. execution_steps
7. shared_contexts
8. a2a_messages
9. human_approvals
10. workflow_templates
```

### 问题 2: 迁移回滚顺序

**描述**: 回滚时外键约束阻止删除

**解决方案**:
按相反顺序删除表

```sql
-- 000003_multi_agent_orchestration.down.sql
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
```

## 验证结果

- [x] 所有表创建成功
- [x] 索引创建正确
- [x] 外键约束生效
- [x] 迁移可正常执行和回滚

## 相关文件

- `backend/migrations/000003_multi_agent_orchestration.up.sql`
- `backend/migrations/000003_multi_agent_orchestration.down.sql`
