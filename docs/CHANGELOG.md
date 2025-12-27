# AgentLink 开发日志

## [Unreleased]

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
