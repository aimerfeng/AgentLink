# 开发日志 014: 试用机制实现

**日期**: 2024-12-28  
**任务**: Task 21 - 实现试用机制  
**状态**: ✅ 已完成

## 任务描述

实现 Agent 试用功能，允许用户在购买前试用 Agent。

## 实现内容

### 1. 试用配置

```go
type TrialConfig struct {
    DefaultTrialCalls int           // 默认试用次数
    TrialExpiry       time.Duration // 试用有效期
    MaxTrialsPerAgent int           // 每个 Agent 最大试用次数
}

var defaultTrialConfig = TrialConfig{
    DefaultTrialCalls: 3,
    TrialExpiry:       24 * time.Hour,
    MaxTrialsPerAgent: 1,
}
```

### 2. 试用服务

```go
type TrialService struct {
    db     *pgxpool.Pool
    config TrialConfig
}

func (s *TrialService) CheckTrialEligibility(ctx context.Context, userID, agentID uuid.UUID) (*TrialStatus, error) {
    // 检查 Agent 是否启用试用
    var trialEnabled bool
    var trialCalls int
    err := s.db.QueryRow(ctx, `
        SELECT trial_enabled, trial_calls FROM agents WHERE id = $1
    `, agentID).Scan(&trialEnabled, &trialCalls)
    
    if err != nil {
        return nil, err
    }
    
    if !trialEnabled {
        return &TrialStatus{Eligible: false, Reason: "trial_disabled"}, nil
    }
    
    // 检查用户是否已使用过试用
    var usedTrials int
    err = s.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(calls_used), 0) 
        FROM trial_usage 
        WHERE user_id = $1 AND agent_id = $2
    `, userID, agentID).Scan(&usedTrials)
    
    if err != nil {
        return nil, err
    }
    
    if usedTrials >= trialCalls {
        return &TrialStatus{
            Eligible:  false,
            Reason:    "trial_exhausted",
            UsedCalls: usedTrials,
            MaxCalls:  trialCalls,
        }, nil
    }
    
    return &TrialStatus{
        Eligible:       true,
        RemainingCalls: trialCalls - usedTrials,
        MaxCalls:       trialCalls,
    }, nil
}

func (s *TrialService) UseTrialCall(ctx context.Context, userID, agentID uuid.UUID) error {
    // 使用 UPSERT 记录试用使用
    _, err := s.db.Exec(ctx, `
        INSERT INTO trial_usage (user_id, agent_id, calls_used, first_used_at, last_used_at)
        VALUES ($1, $2, 1, NOW(), NOW())
        ON CONFLICT (user_id, agent_id) 
        DO UPDATE SET 
            calls_used = trial_usage.calls_used + 1,
            last_used_at = NOW()
    `, userID, agentID)
    
    return err
}
```

### 3. 数据库表

```sql
-- 000005_trial_mechanism.up.sql
CREATE TABLE trial_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    agent_id UUID NOT NULL REFERENCES agents(id),
    calls_used INTEGER DEFAULT 0,
    first_used_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, agent_id)
);

CREATE INDEX idx_trial_usage_user ON trial_usage(user_id);
CREATE INDEX idx_trial_usage_agent ON trial_usage(agent_id);

-- 在 agents 表添加试用配置
ALTER TABLE agents ADD COLUMN trial_enabled BOOLEAN DEFAULT TRUE;
ALTER TABLE agents ADD COLUMN trial_calls INTEGER DEFAULT 3;
```

### 4. Proxy Gateway 集成

```go
func (p *ProxyService) ProcessChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // ... 验证 API Key ...
    
    // 检查配额
    hasQuota, err := p.checkQuota(ctx, apiKey.UserID)
    if err != nil {
        return nil, err
    }
    
    if !hasQuota {
        // 检查试用资格
        trialStatus, err := p.trialService.CheckTrialEligibility(ctx, apiKey.UserID, req.AgentID)
        if err != nil {
            return nil, err
        }
        
        if !trialStatus.Eligible {
            return nil, ErrQuotaExhausted
        }
        
        // 使用试用调用
        if err := p.trialService.UseTrialCall(ctx, apiKey.UserID, req.AgentID); err != nil {
            return nil, err
        }
        
        // 标记为试用调用
        ctx = context.WithValue(ctx, "is_trial", true)
    }
    
    // ... 继续处理 ...
}
```

## 遇到的问题

### 问题 1: 试用次数竞态条件

**描述**: 并发请求可能超过试用限制

**解决方案**:
使用数据库锁或 Redis 原子操作

```go
func (s *TrialService) UseTrialCall(ctx context.Context, userID, agentID uuid.UUID) error {
    // 使用 SELECT FOR UPDATE 锁定行
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    
    var usedCalls, maxCalls int
    err = tx.QueryRow(ctx, `
        SELECT tu.calls_used, a.trial_calls
        FROM trial_usage tu
        JOIN agents a ON a.id = tu.agent_id
        WHERE tu.user_id = $1 AND tu.agent_id = $2
        FOR UPDATE
    `, userID, agentID).Scan(&usedCalls, &maxCalls)
    
    if err == pgx.ErrNoRows {
        // 首次使用，插入记录
        _, err = tx.Exec(ctx, `
            INSERT INTO trial_usage (user_id, agent_id, calls_used)
            VALUES ($1, $2, 1)
        `, userID, agentID)
    } else if err != nil {
        return err
    } else if usedCalls >= maxCalls {
        return ErrTrialExhausted
    } else {
        // 更新使用次数
        _, err = tx.Exec(ctx, `
            UPDATE trial_usage SET calls_used = calls_used + 1, last_used_at = NOW()
            WHERE user_id = $1 AND agent_id = $2
        `, userID, agentID)
    }
    
    if err != nil {
        return err
    }
    
    return tx.Commit(ctx)
}
```

### 问题 2: 试用禁用后已有试用仍可用

**描述**: 创作者禁用试用后，已获得试用的用户仍能使用

**解决方案**:
每次检查时验证 Agent 试用状态

```go
// 检查 Agent 当前是否启用试用
if !agent.TrialEnabled {
    return &TrialStatus{Eligible: false, Reason: "trial_disabled"}, nil
}
```

### 问题 3: 试用统计不准确

**描述**: 试用使用统计与实际不符

**解决方案**:
使用调用日志作为真实来源

```go
// 从调用日志统计试用使用
var actualUsed int
err = s.db.QueryRow(ctx, `
    SELECT COUNT(*) FROM call_logs 
    WHERE user_id = $1 AND agent_id = $2 AND is_trial = true
`, userID, agentID).Scan(&actualUsed)
```

## 验证结果

- [x] 试用资格检查正确
- [x] 试用次数正确扣减
- [x] 试用耗尽后拒绝调用
- [x] 禁用试用后不可使用
- [x] 并发安全

## 属性测试

**Trial Quota Properties**
- 试用次数不超过配置限制 ✅
- 试用耗尽后返回错误 ✅
- 禁用试用后不可使用 ✅
- 并发使用试用次数正确 ✅

## 相关文件

- `backend/internal/trial/trial.go`
- `backend/internal/trial/trial_property_test.go`
- `backend/migrations/000005_trial_mechanism.up.sql`
