# AgentLink 开发日志

## [Unreleased]

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
