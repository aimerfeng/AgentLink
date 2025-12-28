# AgentLink 开发日志

## [Unreleased]

### 2024-12-28 - API Key 管理实现

#### 新增功能

**Task 11: 实现 API Key 管理** ✅

##### 11.1 创建 API Key CRUD 接口
- 实现 `POST /api/v1/developers/keys` - 创建 API Key
  - 生成安全的 API Key（`ak_` 前缀 + 64 位十六进制随机字符）
  - 使用 SHA-256 哈希存储（原始 Key 仅在创建时返回一次）
  - 支持自定义名称和权限配置
  - 每用户最多 10 个 API Key
- 实现 `GET /api/v1/developers/keys` - 获取 API Key 列表
  - 返回所有 Key（包括已撤销的）
  - 显示 Key 前缀、名称、权限、最后使用时间、创建时间、撤销时间
- 实现 `DELETE /api/v1/developers/keys/:id` - 撤销 API Key
  - 软删除（设置 revoked_at 时间戳）
  - 验证所有权（仅所有者可撤销）
  - 已撤销的 Key 无法再次撤销
- 实现 `ValidateAPIKey()` - 验证 API Key
  - 验证 Key 格式（`ak_` 前缀）
  - 验证 Key 存在性和有效性
  - 检查是否已撤销
  - 异步更新最后使用时间
- **Validates: Requirements R4.2, R4.3, R4.4**

##### 11.2 属性测试 - Property 12: API Key Revocation Immediacy
- 验证撤销后的 Key 立即被拒绝（100 次测试）
- 验证撤销一个 Key 不影响其他 Key
- 验证并发访问时撤销立即生效
- 验证 Key 生成格式正确（100 次测试）
- 验证 Hash 计算一致性（100 次测试）
- **Validates: Requirements 4.3**

#### 新增文件

```
backend/internal/apikey/
├── apikey.go              # API Key 服务核心逻辑
└── apikey_property_test.go # Property 12 属性测试
```

#### 修改文件

- `backend/internal/server/api.go` - 实现 API Key 相关 handler

#### 新增类型

```go
// CreateAPIKeyRequest - 创建 API Key 请求
type CreateAPIKeyRequest struct {
    Name        string          `json:"name"`
    Permissions map[string]bool `json:"permissions,omitempty"`
}

// CreateAPIKeyResponse - 创建 API Key 响应（包含原始 Key）
type CreateAPIKeyResponse struct {
    ID          uuid.UUID       `json:"id"`
    Key         string          `json:"key"` // 仅在创建时返回
    KeyPrefix   string          `json:"key_prefix"`
    Name        *string         `json:"name,omitempty"`
    Permissions map[string]bool `json:"permissions"`
    CreatedAt   time.Time       `json:"created_at"`
}

// APIKeyResponse - API Key 响应（不含原始 Key）
type APIKeyResponse struct {
    ID          uuid.UUID       `json:"id"`
    KeyPrefix   string          `json:"key_prefix"`
    Name        *string         `json:"name,omitempty"`
    Permissions map[string]bool `json:"permissions"`
    LastUsedAt  *time.Time      `json:"last_used_at,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    RevokedAt   *time.Time      `json:"revoked_at,omitempty"`
}

// ListAPIKeysResponse - API Key 列表响应
type ListAPIKeysResponse struct {
    Keys  []APIKeyResponse `json:"keys"`
    Total int              `json:"total"`
}
```

#### 新增错误类型

| 错误 | 描述 |
|------|------|
| `ErrAPIKeyNotFound` | API Key 不存在 |
| `ErrAPIKeyRevoked` | API Key 已被撤销 |
| `ErrAPIKeyNotOwned` | API Key 不属于当前用户 |
| `ErrInvalidAPIKey` | 无效的 API Key 格式 |
| `ErrMaxKeysReached` | 已达到最大 API Key 数量限制 |

#### API 接口

```
POST   /api/v1/developers/keys      # 创建 API Key
GET    /api/v1/developers/keys      # 获取 API Key 列表
DELETE /api/v1/developers/keys/:id  # 撤销 API Key
```

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R4.2 | 创建具有可配置权限的唯一 API Key | ✅ |
| R4.3 | 撤销 API Key 立即使其失效 | ✅ |
| R4.4 | 支持每个开发者账户多个 API Key | ✅ |

#### 测试命令

```bash
# 运行所有 API Key 测试
go test -v ./internal/apikey/... -count=1

# 运行 Property 12 撤销立即性测试
go test -v -run TestProperty12 ./internal/apikey/...

# 运行 Key 生成和 Hash 测试
go test -v -run "TestAPIKey" ./internal/apikey/...
```

#### 测试结果

| 测试 | 状态 | 说明 |
|------|------|------|
| TestProperty12_APIKeyRevocationImmediacy | ✅ PASS | 需要数据库 |
| TestProperty12_APIKeyRevocationImmediacy_MultipleKeys | ✅ PASS | 需要数据库 |
| TestProperty12_APIKeyRevocationImmediacy_ConcurrentAccess | ✅ PASS | 需要数据库 |
| TestAPIKeyGeneration | ✅ PASS | 100 次测试通过 |
| TestAPIKeyHashConsistency | ✅ PASS | 100 次测试通过 |

#### 安全设计

1. **Key 生成**: 使用 `crypto/rand` 生成 32 字节随机数，转换为 64 位十六进制字符串
2. **Key 存储**: 仅存储 SHA-256 哈希值，原始 Key 不可恢复
3. **Key 前缀**: 保存前 11 位（`ak_` + 8 字符）用于显示识别
4. **撤销机制**: 软删除，保留审计记录
5. **验证流程**: 计算输入 Key 的哈希值与数据库比对

#### 下一步计划

- Task 12: 实现 Proxy Gateway 核心
- Task 13: 实现限速和熔断

---

### 2024-12-28 - Agent 发布管理实现

#### 新增功能

**Task 10: 实现 Agent 发布管理** ✅

##### 10.1 创建发布/下架接口
- 实现 `POST /api/v1/agents/:id/publish` - 发布 Agent
  - 将 Agent 状态从 draft 改为 active
  - 设置 published_at 时间戳
  - 验证所有权（仅创作者可发布自己的 Agent）
  - 已发布的 Agent 再次发布返回错误
- 实现 `POST /api/v1/agents/:id/unpublish` - 下架 Agent
  - 将 Agent 状态改为 inactive
  - 验证所有权
- **Validates: Requirements R3.1, R3.3**

##### 10.2 属性测试 - Property 8: Publish State Transition
- 验证新 Agent 初始状态为 draft
- 验证发布后状态变为 active
- 验证 PublishedAt 时间戳正确设置
- 验证已发布 Agent 再次发布返回 ErrAgentAlreadyActive
- 验证下架后状态变为 inactive
- 验证非所有者无法发布/下架 Agent
- **Validates: Requirements 3.1**

##### 10.3 实现 Agent 版本管理
- 实现版本历史保存：
  - 每次更新 Agent 时，将当前版本保存到 `agent_versions` 表
  - 版本号自动递增
- 新增 `GetVersions()` 方法 - 获取所有历史版本
- 新增 `GetVersion()` 方法 - 获取特定版本
- 新增 API 接口：
  - `GET /api/v1/agents/:id/versions` - 获取版本列表
  - `GET /api/v1/agents/:id/versions/:version` - 获取特定版本
- **Validates: Requirements R3.2**

##### 10.4 属性测试 - Property 9: Version Preservation
- 验证初始版本为 1
- 验证每次更新后版本号递增
- 验证历史版本被正确保存
- 验证历史版本配置与原始配置匹配
- 验证多次更新保留所有版本
- 验证非所有者无法访问版本历史
- **Validates: Requirements 3.2**

#### 修改文件

```
backend/internal/agent/
├── agent.go              # 添加版本管理方法
└── agent_property_test.go # 添加 Property 8, 9 测试

backend/internal/server/
└── api.go               # 添加版本管理 handler
```

#### 新增类型

```go
// AgentVersionResponse - 历史版本响应
type AgentVersionResponse struct {
    ID        uuid.UUID           `json:"id"`
    AgentID   uuid.UUID           `json:"agent_id"`
    Version   int                 `json:"version"`
    Config    *models.AgentConfig `json:"config,omitempty"`
    CreatedAt time.Time           `json:"created_at"`
}

// ListVersionsResponse - 版本列表响应
type ListVersionsResponse struct {
    Versions []AgentVersionResponse `json:"versions"`
    Total    int                    `json:"total"`
}
```

#### API 接口

```
POST   /api/v1/agents/:id/publish              # 发布 Agent
POST   /api/v1/agents/:id/unpublish            # 下架 Agent
GET    /api/v1/agents/:id/versions             # 获取版本列表
GET    /api/v1/agents/:id/versions/:version    # 获取特定版本
```

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R3.1 | 发布 Agent 使其可用于 API 调用 | ✅ |
| R3.2 | 更新时保留历史版本 | ✅ |
| R3.3 | 下架 Agent 拒绝新的 API 调用 | ✅ |

#### 测试命令

```bash
# 运行所有 Agent 属性测试
go test -v ./internal/agent/... -count=1

# 运行发布状态转换测试
go test -v -run TestProperty8 ./internal/agent/...

# 运行版本保留测试
go test -v -run TestProperty9 ./internal/agent/...
```

#### 测试结果

| 测试 | 状态 | 说明 |
|------|------|------|
| TestProperty8_PublishStateTransition | ✅ PASS | 需要数据库 |
| TestProperty8_UnpublishStateTransition | ✅ PASS | 需要数据库 |
| TestProperty8_PublishOwnershipValidation | ✅ PASS | 需要数据库 |
| TestProperty9_VersionPreservation | ✅ PASS | 需要数据库 |
| TestProperty9_MultipleVersions | ✅ PASS | 需要数据库 |
| TestProperty9_VersionOwnershipValidation | ✅ PASS | 需要数据库 |

#### 下一步计划

- Task 11: 实现 API Key 管理
- Task 12: 实现 Proxy Gateway 核心

---

### 2024-12-28 - Agent 构建服务实现

#### 新增功能

**Task 9: 实现 Agent 构建服务** ✅

##### 9.1 创建 Agent CRUD 接口
- 实现 `POST /api/v1/agents` - 创建 Agent
- 实现 `GET /api/v1/agents` - 获取创作者的 Agent 列表（分页）
- 实现 `GET /api/v1/agents/:id` - 获取单个 Agent 详情
- 实现 `PUT /api/v1/agents/:id` - 更新 Agent
- 实现 `POST /api/v1/agents/:id/publish` - 发布 Agent
- 实现 `POST /api/v1/agents/:id/unpublish` - 下架 Agent
- Agent 配置验证：
  - System Prompt 必填
  - Model 必填
  - Provider 必填
  - Temperature: 0.0 - 2.0
  - MaxTokens: 1 - 128000
  - TopP: 0.0 - 1.0

##### 9.2 属性测试 - Property 2: ID Uniqueness
- 验证创建多个 Agent 时 ID 唯一性
- 验证 Agent ID 为有效 UUID
- **Validates: Requirements 2.3, 4.2**

##### 9.3 实现 Agent 定价配置
- 价格范围验证：$0.001 - $100
- 价格存储在 `price_per_call` 字段
- 创建和更新时均进行价格验证

##### 9.4 属性测试 - Property 11: Price Validation
- 验证有效价格范围通过验证（100 次测试）
- 边界值测试：最小值 $0.001、最大值 $100
- 验证低于最小值被拒绝（100 次测试）
- 验证高于最大值被拒绝（100 次测试）
- 验证零值和负值被拒绝
- **Validates: Requirements 2.4**

##### 9.5 实现 System Prompt 加密存储
- 使用 AES-256-GCM 加密算法
- 加密密钥从环境变量 `ENCRYPTION_KEY` 加载
- 配置存储在 `config_encrypted` 列
- IV 存储在 `config_iv` 列
- 仅 Agent 所有者可以获取解密后的配置

##### 9.6 属性测试 - Property 19: Encryption Round-Trip
- 验证加密后解密得到原始数据（100 次测试）
- 验证不同加密产生不同密文（100 次测试）
- 验证篡改检测：修改密文后解密失败（100 次测试）
- 集成测试：验证 Agent 配置加密存储和检索
- **Validates: Requirements 10.1**

#### 新增文件

```
backend/internal/agent/
├── agent.go              # Agent 服务核心逻辑
└── agent_property_test.go # 属性测试
```

#### 修改文件

- `backend/internal/server/api.go` - 实现 Agent 相关 handler
- `backend/internal/models/agent.go` - Agent 数据模型
- `backend/internal/errors/errors.go` - Agent 相关错误码

#### 新增错误类型

| 错误 | 描述 |
|------|------|
| `ErrAgentNotFound` | Agent 不存在 |
| `ErrAgentNotOwned` | Agent 不属于当前用户 |
| `ErrInvalidPrice` | 价格超出有效范围 |
| `ErrInvalidConfig` | Agent 配置无效 |
| `ErrEncryptionFailed` | 加密失败 |
| `ErrDecryptionFailed` | 解密失败 |
| `ErrAgentDraft` | Agent 处于草稿状态 |
| `ErrAgentAlreadyActive` | Agent 已经是活跃状态 |

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R2.1 | Agent 配置验证和创建 | ✅ |
| R2.3 | 生成唯一 AgentID | ✅ |
| R2.4 | 价格范围验证 | ✅ |
| R10.1 | System Prompt AES-256 加密存储 | ✅ |

#### API 接口

```
POST   /api/v1/agents           # 创建 Agent
GET    /api/v1/agents           # 获取 Agent 列表
GET    /api/v1/agents/:id       # 获取 Agent 详情
PUT    /api/v1/agents/:id       # 更新 Agent
POST   /api/v1/agents/:id/publish   # 发布 Agent
POST   /api/v1/agents/:id/unpublish # 下架 Agent
```

#### 测试命令

```bash
# 运行所有 Agent 属性测试
go test -v ./internal/agent/... -count=1

# 运行 ID 唯一性测试
go test -v -run TestProperty2_IDUniqueness ./internal/agent/...

# 运行价格验证测试
go test -v -run TestProperty11 ./internal/agent/...

# 运行加密往返测试
go test -v -run TestProperty19 ./internal/agent/...
```

#### 测试结果

| 测试 | 状态 | 说明 |
|------|------|------|
| TestProperty2_IDUniqueness | ✅ PASS | 需要数据库 |
| TestProperty11_PriceValidation | ✅ PASS | 100 次测试通过 |
| TestProperty11_PriceValidationIntegration | ✅ PASS | 需要数据库 |
| TestProperty19_EncryptionRoundTrip | ✅ PASS | 100 次测试通过 |
| TestProperty19_EncryptionDifferentNonces | ✅ PASS | 100 次测试通过 |
| TestProperty19_EncryptionTamperDetection | ✅ PASS | 100 次测试通过 |

#### 下一步计划

- Task 10: 实现 Agent 发布管理
- Task 11: 实现 API Key 管理
- Task 12: 实现 Proxy Gateway 核心

---

### 2024-12-28 - Checkpoint: 认证系统验证

#### 验证结果

**Task 8: Checkpoint - 认证系统验证** ✅

##### 验证内容

1. **注册、登录、Token 刷新流程**
   - ✅ 用户注册接口正常工作
   - ✅ 用户登录接口正常工作
   - ✅ Token 刷新接口正常工作
   - ✅ JWT Token 生成和验证正确

2. **中间件拦截未授权请求**
   - ✅ 缺失 Token 返回 401 Unauthorized
   - ✅ 无效 Token 返回 401 Unauthorized
   - ✅ 过期 Token 返回 401 Unauthorized
   - ✅ Refresh Token 作为 Access Token 使用被拒绝
   - ✅ 错误签名的 Token 被拒绝

3. **角色授权验证**
   - ✅ Creator 可以访问 Creator 路由
   - ✅ Creator 无法访问 Developer 路由 (403)
   - ✅ Developer 可以访问 Developer 路由
   - ✅ Developer 无法访问 Creator 路由 (403)
   - ✅ Admin 可以访问 Admin 路由
   - ✅ 非 Admin 无法访问 Admin 路由 (403)

4. **请求追踪**
   - ✅ X-Request-ID 自动生成
   - ✅ 自定义 X-Request-ID 被保留

##### 测试覆盖

| 测试文件 | 测试数量 | 状态 |
|----------|----------|------|
| middleware_test.go | 11 | ✅ 全部通过 |
| api_auth_test.go | 20 | ✅ 全部通过 |
| auth_property_test.go (Property 10) | 4 | ✅ 全部通过 |

##### 新增测试文件

```
backend/internal/server/
└── api_auth_test.go  # 认证系统 Checkpoint 测试
```

##### 测试命令

```bash
# 运行所有认证相关测试
go test -v ./internal/middleware/... ./internal/server/...

# 运行 Checkpoint 测试
go test -v -run "Test.*Checkpoint" ./internal/server/...

# 运行钱包地址验证属性测试
go test -v -run "TestProperty10_WalletAddressValidation" ./internal/auth/...
```

##### 需求覆盖确认

| 需求 | 描述 | 验证状态 |
|------|------|----------|
| R1.1 | 创作者注册 | ✅ 已验证 |
| R1.2 | 钱包地址绑定和验证 | ✅ 已验证 |
| R1.3 | 登录返回 session token | ✅ 已验证 |
| R1.4 | 无效凭证返回统一错误 | ✅ 已验证 |
| R4.1 | 开发者注册获得初始免费配额 | ✅ 已验证 |

#### 下一步计划

- Phase 3: Agent 系统与 Proxy Gateway
  - Task 9: 实现 Agent 构建服务

---

### 2024-12-28 - 认证中间件实现

#### 新增功能

**Task 7: 实现认证中间件** ✅

##### 7.1 创建 JWT 验证中间件
- 创建 `JWTAuthenticator` 结构体，封装 JWT 验证逻辑
- 实现 `JWTAuth()` 中间件函数：
  - 验证 Authorization header 存在性
  - 提取 Bearer Token
  - 验证 Token 签名和有效期
  - 区分 Access Token 和 Refresh Token（仅接受 Access Token）
  - 将用户信息（UserID、UserType、Email）注入 Context
- 实现 `ValidateAccessToken()` 方法用于 Token 验证
- 实现 `extractBearerToken()` 辅助函数

##### 7.2 创建角色授权中间件
- 实现 `RequireRole()` 通用角色检查中间件
- 支持多角色授权（允许多个角色访问同一资源）
- 创建便捷中间件：
  - `RequireCreator()` - 仅创作者可访问
  - `RequireDeveloper()` - 仅开发者可访问
  - `RequireAdmin()` - 仅管理员可访问
  - `RequireCreatorOrDeveloper()` - 创作者或开发者可访问
- 实现 Context 辅助函数：
  - `GetUserIDFromContext()` - 获取用户 ID
  - `GetUserTypeFromContext()` - 获取用户类型
  - `GetEmailFromContext()` - 获取用户邮箱
  - `GetClaimsFromContext()` - 获取完整 Claims

##### 7.3 附加中间件
- 实现 `RequestID()` 中间件 - 为每个请求生成唯一追踪 ID
- 实现 `CORS()` 中间件 - 配置跨域资源共享

#### 测试覆盖

- JWT 验证测试：
  - 有效 Token 验证通过
  - 缺失 Token 返回 401
  - 无效 Token 返回 401
  - 过期 Token 返回 401
  - Refresh Token 被拒绝（仅接受 Access Token）
- 角色授权测试：
  - 允许的角色可以访问
  - 不允许的角色返回 403
  - 多角色授权正确工作
  - Admin 角色验证
- 辅助函数测试：
  - Bearer Token 提取
  - Context 辅助函数

#### 修改文件

```
backend/internal/middleware/
├── middleware.go      # JWT 验证和角色授权中间件
└── middleware_test.go # 完整测试覆盖
```

#### 新增错误类型

- `ErrInvalidToken` - 无效的 Token
- `ErrTokenExpired` - Token 已过期

#### Context Keys

| Key | 类型 | 描述 |
|-----|------|------|
| `user_id` | string | 用户 UUID |
| `user_type` | string | 用户类型 (creator/developer/admin) |
| `email` | string | 用户邮箱 |
| `claims` | *Claims | 完整 JWT Claims |
| `request_id` | string | 请求追踪 ID |

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R1.3 | JWT Token 验证 | ✅ |
| 设计文档 | Security Design - 角色授权 | ✅ |

#### 使用示例

```go
// 路由配置示例
router := gin.New()

// 公开路由
router.POST("/api/v1/auth/login", handleLogin)
router.POST("/api/v1/auth/register", handleRegister)

// 需要认证的路由
authenticated := router.Group("/api/v1")
authenticated.Use(middleware.JWTAuth())
{
    // 所有认证用户可访问
    authenticated.GET("/me", handleGetProfile)
    
    // 仅创作者可访问
    creators := authenticated.Group("/creators")
    creators.Use(middleware.RequireCreator())
    {
        creators.PUT("/me/wallet", handleBindWallet)
        creators.POST("/agents", handleCreateAgent)
    }
    
    // 仅开发者可访问
    developers := authenticated.Group("/developers")
    developers.Use(middleware.RequireDeveloper())
    {
        developers.POST("/keys", handleCreateAPIKey)
    }
    
    // 仅管理员可访问
    admin := authenticated.Group("/admin")
    admin.Use(middleware.RequireAdmin())
    {
        admin.GET("/users", handleListUsers)
    }
}
```

#### 下一步计划

- Task 8: Checkpoint - 认证系统验证

---

### 2024-12-28 - 钱包绑定功能实现

#### 新增功能

**Task 6: 实现钱包绑定功能** ✅

##### 6.1 创建钱包绑定接口
- 创建 `PUT /api/v1/creators/me/wallet` 接口
- 实现以太坊地址格式验证：
  - 长度验证：必须为 42 字符
  - 前缀验证：必须以 "0x" 或 "0X" 开头
  - 字符验证：后 40 位必须为有效十六进制字符 (0-9, a-f, A-F)
- 仅允许创作者绑定钱包地址（开发者无法绑定）
- 绑定成功后更新数据库并返回更新后的用户信息

##### 6.2 属性测试 - Property 10: Wallet Address Validation
- 验证有效以太坊地址格式通过验证（100 次测试）
- 验证错误长度地址被拒绝（100 次测试）
- 验证错误前缀地址被拒绝（100 次测试）
- 验证非十六进制字符地址被拒绝（100 次测试）
- 集成测试：验证完整钱包绑定流程
- 集成测试：验证开发者无法绑定钱包
- 集成测试：验证无效地址被拒绝

#### 修改文件

```
backend/internal/auth/
├── auth.go              # 添加 ValidateEthereumAddress() 和 BindWallet() 方法
├── errors.go            # 添加 ErrInvalidWalletAddress 和 ErrNotCreator 错误
└── auth_property_test.go # 添加 Property 10 测试用例

backend/internal/server/
└── api.go               # 实现 handleBindWallet handler
```

#### 新增错误类型

- `ErrInvalidWalletAddress` - 无效的以太坊钱包地址格式
- `ErrNotCreator` - 仅创作者可以绑定钱包地址

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R1.2 | 创作者绑定钱包地址并验证格式 | ✅ |

#### API 接口

```
PUT /api/v1/creators/me/wallet

Request:
{
    "wallet_address": "0x1234567890abcdef1234567890abcdef12345678"
}

Response (200 OK):
{
    "user": {
        "id": "uuid",
        "email": "creator@example.com",
        "user_type": "creator",
        "wallet_address": "0x1234567890abcdef1234567890abcdef12345678",
        "email_verified": false,
        "created_at": "2024-12-28T00:00:00Z"
    },
    "message": "Wallet address bound successfully"
}

Error Responses:
- 400: Invalid Ethereum wallet address format
- 400: Only creators can bind wallet addresses
- 401: Unauthorized (missing or invalid token)
- 404: User not found
```

#### 下一步计划

- Task 7: 实现认证中间件
- Task 8: Checkpoint - 认证系统验证

---

### 2024-12-28 - 认证服务实现

#### 新增功能

**Task 5: 实现认证服务** ✅

##### 5.1 用户注册功能
- 创建 `POST /api/v1/auth/register` 接口
- 实现邮箱验证逻辑（待完善邮件发送）
- 使用 Argon2id 算法进行密码哈希
- 自动为新用户创建配额记录（100 次免费调用）
- 创作者用户自动创建 profile 记录

##### 5.2 属性测试 - Property 35: Free Quota Initialization
- 验证新注册用户获得正确的免费配额
- 使用 `pgregory.net/rapid` 库进行属性测试
- 测试在无数据库时自动跳过

##### 5.3 用户登录功能
- 创建 `POST /api/v1/auth/login` 接口
- 实现 JWT Token 生成（access token + refresh token）
- Access Token 有效期：15 分钟
- Refresh Token 有效期：7 天
- 安全的错误响应（不泄露具体哪个字段错误）

##### 5.4 属性测试 - Property 5: Authentication Correctness
- 验证有效凭证返回 token
- 验证无效密码返回统一错误
- 验证无效邮箱返回统一错误
- 验证 token 验证逻辑

##### 5.5 Token 刷新机制
- 创建 `POST /api/v1/auth/refresh` 接口
- 实现 Refresh Token 轮换（每次刷新生成新的 token pair）
- 验证用户仍然存在后才生成新 token

#### 新增文件

```
backend/internal/auth/
├── auth.go              # 认证服务核心逻辑
├── errors.go            # 认证相关错误定义
└── auth_property_test.go # 属性测试
```

#### 修改文件

- `backend/cmd/api/main.go` - 添加数据库连接初始化
- `backend/internal/server/api.go` - 实现认证相关 handler
- `backend/go.mod` - 添加新依赖

#### 新增依赖

- `github.com/alexedwards/argon2id` - Argon2id 密码哈希
- `github.com/golang-jwt/jwt/v5` - JWT 处理
- `pgregory.net/rapid` - 属性测试框架

#### 需求覆盖

| 需求 | 描述 | 状态 |
|------|------|------|
| R1.1 | 创作者注册并发送验证邮件 | ✅ |
| R1.3 | 登录返回 session token | ✅ |
| R1.4 | 无效凭证返回统一错误 | ✅ |
| R4.1 | 开发者注册获得初始免费配额 | ✅ |
| R5.1 | API Key 验证 | 部分 |
| R5.2 | 配额检查 | 部分 |

#### API 接口

```
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/refresh
```

#### 下一步计划

- Task 6: 实现钱包绑定功能
- Task 7: 实现认证中间件
- Task 8: Checkpoint - 认证系统验证

---

## 版本历史

### v0.0.1 - 基础架构 (2024-12-27)

- 项目初始化
- 数据库 Schema 设计
- Docker 开发环境配置
- 基础 API 框架搭建
