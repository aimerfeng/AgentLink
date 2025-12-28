# 开发日志 008: Agent CRUD 接口

**日期**: 2024-12-28  
**任务**: Task 9.1 - 创建 Agent CRUD 接口  
**状态**: ✅ 已完成

## 任务描述

实现 Agent 的创建、读取、更新、删除接口，包括配置验证和加密存储。

## 实现内容

### 1. API 接口

```
POST   /api/v1/agents           # 创建 Agent
GET    /api/v1/agents           # 获取 Agent 列表
GET    /api/v1/agents/:id       # 获取 Agent 详情
PUT    /api/v1/agents/:id       # 更新 Agent
DELETE /api/v1/agents/:id       # 删除 Agent
```

### 2. Agent 配置结构

```go
type AgentConfig struct {
    SystemPrompt string  `json:"system_prompt"`
    Model        string  `json:"model"`
    Provider     string  `json:"provider"`
    Temperature  float64 `json:"temperature"`
    MaxTokens    int     `json:"max_tokens"`
    TopP         float64 `json:"top_p"`
}

type CreateAgentRequest struct {
    Name         string       `json:"name" binding:"required"`
    Description  string       `json:"description"`
    Config       AgentConfig  `json:"config" binding:"required"`
    PricePerCall float64      `json:"price_per_call" binding:"required"`
}
```

### 3. 配置验证

```go
func validateAgentConfig(config *AgentConfig) error {
    // System Prompt 必填
    if strings.TrimSpace(config.SystemPrompt) == "" {
        return ErrSystemPromptRequired
    }
    
    // Model 必填
    if strings.TrimSpace(config.Model) == "" {
        return ErrModelRequired
    }
    
    // Provider 必填
    validProviders := map[string]bool{
        "openai": true, "anthropic": true, "google": true,
    }
    if !validProviders[config.Provider] {
        return ErrInvalidProvider
    }
    
    // Temperature: 0.0 - 2.0
    if config.Temperature < 0 || config.Temperature > 2 {
        return ErrInvalidTemperature
    }
    
    // MaxTokens: 1 - 128000
    if config.MaxTokens < 1 || config.MaxTokens > 128000 {
        return ErrInvalidMaxTokens
    }
    
    // TopP: 0.0 - 1.0
    if config.TopP < 0 || config.TopP > 1 {
        return ErrInvalidTopP
    }
    
    return nil
}
```

### 4. 创建 Agent

```go
func (s *AgentService) Create(ctx context.Context, creatorID uuid.UUID, req CreateAgentRequest) (*Agent, error) {
    // 1. 验证配置
    if err := validateAgentConfig(&req.Config); err != nil {
        return nil, err
    }
    
    // 2. 验证价格
    if err := validatePrice(req.PricePerCall); err != nil {
        return nil, err
    }
    
    // 3. 加密配置
    configJSON, _ := json.Marshal(req.Config)
    encrypted, iv, err := s.encrypt(configJSON)
    if err != nil {
        return nil, ErrEncryptionFailed
    }
    
    // 4. 插入数据库
    agent := &Agent{
        CreatorID:       creatorID,
        Name:            req.Name,
        Description:     req.Description,
        Status:          "draft",
        PricePerCall:    decimal.NewFromFloat(req.PricePerCall),
        ConfigEncrypted: encrypted,
        ConfigIV:        iv,
        Version:         1,
    }
    
    err = s.db.QueryRow(ctx, insertAgentSQL, ...).Scan(&agent.ID, ...)
    if err != nil {
        return nil, err
    }
    
    return agent, nil
}
```

## 遇到的问题

### 问题 1: JSON 序列化精度丢失

**描述**: float64 价格在 JSON 序列化时精度丢失

**示例**:
```json
// 输入
{"price_per_call": 0.001}

// 实际存储
0.0009999999999999998
```

**解决方案**:
使用 `shopspring/decimal` 库处理精确小数

```go
import "github.com/shopspring/decimal"

type Agent struct {
    PricePerCall decimal.Decimal `json:"price_per_call"`
}

// 创建时
agent.PricePerCall = decimal.NewFromFloat(req.PricePerCall)

// JSON 序列化自动处理精度
```

### 问题 2: 配置加密失败

**描述**: 加密密钥长度不正确

**错误信息**:
```
crypto/aes: invalid key size 24
```

**解决方案**:
确保 ENCRYPTION_KEY 为 32 字节（AES-256）

```bash
# 生成 32 字节密钥
openssl rand -hex 32
# 输出: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4
```

### 问题 3: 所有权验证缺失

**描述**: 用户可以修改他人的 Agent

**解决方案**:
在所有操作中验证所有权

```go
func (s *AgentService) Update(ctx context.Context, userID, agentID uuid.UUID, req UpdateAgentRequest) (*Agent, error) {
    // 获取 Agent
    agent, err := s.GetByID(ctx, agentID)
    if err != nil {
        return nil, err
    }
    
    // 验证所有权
    if agent.CreatorID != userID {
        return nil, ErrAgentNotOwned
    }
    
    // ... 更新逻辑
}
```

### 问题 4: 分页参数验证

**描述**: 负数页码导致 SQL 错误

**解决方案**:
验证并规范化分页参数

```go
func normalizePagination(page, pageSize int) (int, int) {
    if page < 1 {
        page = 1
    }
    if pageSize < 1 {
        pageSize = 10
    }
    if pageSize > 100 {
        pageSize = 100
    }
    return page, pageSize
}
```

## 验证结果

- [x] 创建 Agent 成功
- [x] 获取 Agent 列表（分页）
- [x] 获取 Agent 详情
- [x] 更新 Agent 成功
- [x] 删除 Agent 成功
- [x] 配置验证正确
- [x] 所有权验证正确

## 属性测试

**Property 2: ID Uniqueness**
- 创建多个 Agent 时 ID 唯一 ✅
- Agent ID 为有效 UUID ✅

## 相关文件

- `backend/internal/agent/agent.go`
- `backend/internal/agent/agent_property_test.go`
- `backend/internal/models/agent.go`
- `backend/internal/server/api.go`
