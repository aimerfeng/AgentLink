# Design Document: AgentLink Platform

## Overview

AgentLink 是一个 "Prompt 即资产，API 即服务" 的 SaaS 平台。本设计文档描述了系统的整体架构、核心组件、数据模型和关键技术决策。

### 核心设计原则

1. **安全优先**: Prompt 永不暴露，所有敏感数据加密存储
2. **高性能**: Go 后端 + Redis 缓存，支持高并发 API 转发
3. **可扩展**: 微服务架构，支持水平扩展
4. **透明可信**: 区块链存证，收益分成链上可查

## Architecture

### 系统架构图

```mermaid
graph TB
    subgraph "Frontend (Next.js 14)"
        LP[Landing Page]
        MP[Marketplace]
        CD[Creator Dashboard]
        DD[Developer Console]
        AD[Admin Dashboard]
    end

    subgraph "API Layer (Go/Gin)"
        AG[API Gateway]
        PG[Proxy Gateway]
        WS[WebSocket Server]
    end

    subgraph "Core Services"
        AS[Auth Service]
        ABS[Agent Builder Service]
        PS[Payment Service]
        RS[RAG Service]
        NS[Notification Service]
        ANS[Analytics Service]
    end

    subgraph "Data Layer"
        PDB[(PostgreSQL)]
        RDS[(Redis)]
        VDB[(Vector DB)]
        S3[(Object Storage)]
    end

    subgraph "External Services"
        AI[AI Providers<br/>OpenAI/Claude/Gemini]
        ST[Stripe]
        CB[Coinbase Commerce]
        BC[Blockchain<br/>Base/Polygon]
    end

    LP --> AG
    MP --> AG
    CD --> AG
    DD --> AG
    AD --> AG

    AG --> AS
    AG --> ABS
    AG --> PS
    AG --> ANS
    
    PG --> AI
    PG --> RS
    PG --> RDS

    AS --> PDB
    ABS --> PDB
    ABS --> VDB
    PS --> ST
    PS --> CB
    PS --> BC
    RS --> VDB
    RS --> S3
    ANS --> PDB
```

### 部署架构

```mermaid
graph LR
    subgraph "CDN Layer"
        CF[Cloudflare CDN]
    end

    subgraph "Load Balancer"
        LB[Nginx/ALB]
    end

    subgraph "Application Tier"
        FE1[Next.js Pod 1]
        FE2[Next.js Pod 2]
        BE1[Go API Pod 1]
        BE2[Go API Pod 2]
        PG1[Proxy Gateway 1]
        PG2[Proxy Gateway 2]
    end

    subgraph "Data Tier"
        PDB[(PostgreSQL<br/>Primary)]
        PDBR[(PostgreSQL<br/>Replica)]
        RDS[(Redis Cluster)]
    end

    CF --> LB
    LB --> FE1
    LB --> FE2
    LB --> BE1
    LB --> BE2
    LB --> PG1
    LB --> PG2

    BE1 --> PDB
    BE2 --> PDB
    PDB --> PDBR
    PG1 --> RDS
    PG2 --> RDS
```

## Components and Interfaces

### 1. API Gateway (Go/Gin)

负责所有非 AI 调用的 API 请求处理。

```go
// 路由结构
/api/v1/
├── auth/
│   ├── POST   /register          // 用户注册
│   ├── POST   /login             // 用户登录
│   ├── POST   /logout            // 用户登出
│   └── POST   /refresh           // 刷新 Token
├── creators/
│   ├── GET    /me                // 获取当前创作者信息
│   ├── PUT    /me                // 更新创作者信息
│   └── PUT    /me/wallet         // 绑定钱包地址
├── agents/
│   ├── POST   /                  // 创建 Agent
│   ├── GET    /                  // 获取创作者的 Agent 列表
│   ├── GET    /:id               // 获取 Agent 详情
│   ├── PUT    /:id               // 更新 Agent
│   ├── POST   /:id/publish       // 发布 Agent
│   ├── POST   /:id/unpublish     // 下架 Agent
│   └── POST   /:id/knowledge     // 上传知识库
├── marketplace/
│   ├── GET    /agents            // 搜索 Agent
│   ├── GET    /agents/:id        // 获取公开 Agent 详情
│   ├── GET    /categories        // 获取分类列表
│   └── GET    /featured          // 获取推荐 Agent
├── developers/
│   ├── GET    /me                // 获取开发者信息
│   ├── GET    /keys              // 获取 API Key 列表
│   ├── POST   /keys              // 创建 API Key
│   ├── DELETE /keys/:id          // 删除 API Key
│   └── GET    /usage             // 获取使用统计
├── payments/
│   ├── POST   /checkout          // 创建支付会话
│   ├── POST   /webhook/stripe    // Stripe Webhook
│   ├── POST   /webhook/coinbase  // Coinbase Webhook
│   └── GET    /history           // 支付历史
├── reviews/
│   ├── POST   /agents/:id        // 提交评价
│   └── GET    /agents/:id        // 获取评价列表
└── webhooks/
    ├── GET    /                  // 获取 Webhook 配置
    ├── POST   /                  // 创建 Webhook
    └── DELETE /:id               // 删除 Webhook
```

### 2. Proxy Gateway (核心模块)

处理 AI API 调用的代理网关，是系统最关键的组件。

```go
// Proxy Gateway 接口
/proxy/v1/
└── POST /agents/:agentId/chat    // AI 对话接口

// 请求头
X-AgentLink-Key: <api_key>        // 必需：API Key
X-Request-ID: <uuid>              // 可选：请求追踪 ID

// 请求体
{
    "messages": [
        {"role": "user", "content": "Hello"}
    ],
    "stream": true                 // 是否启用流式响应
}

// 响应 (非流式)
{
    "id": "chat-xxx",
    "choices": [
        {"message": {"role": "assistant", "content": "..."}}
    ],
    "usage": {"prompt_tokens": 10, "completion_tokens": 50}
}

// 响应 (流式 SSE)
data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
```

#### Proxy Gateway 处理流程

```mermaid
sequenceDiagram
    participant D as Developer App
    participant PG as Proxy Gateway
    participant RD as Redis
    participant DB as PostgreSQL
    participant AI as AI Provider

    D->>PG: POST /proxy/v1/agents/:id/chat
    PG->>RD: 验证 API Key & 检查配额
    
    alt API Key 无效或配额不足
        PG-->>D: 401/429 Error
    end

    PG->>RD: 检查限速 (Rate Limit)
    
    alt 超过限速
        PG-->>D: 429 Too Many Requests
    end

    PG->>DB: 获取 Agent 配置 (System Prompt)
    PG->>PG: 注入 System Prompt
    
    alt Agent 有知识库
        PG->>VDB: RAG 检索相关上下文
        PG->>PG: 注入 RAG 上下文
    end

    PG->>AI: 转发请求 (带隐藏 Prompt)
    
    alt 流式响应
        loop SSE Events
            AI-->>PG: data: {...}
            PG-->>D: data: {...}
        end
    else 非流式响应
        AI-->>PG: JSON Response
        PG-->>D: JSON Response
    end

    PG->>RD: 扣减配额
    PG->>DB: 记录调用日志 (异步)
```

### 3. Auth Service

处理用户认证和授权。

```go
// JWT Token 结构
type Claims struct {
    UserID   string `json:"user_id"`
    UserType string `json:"user_type"` // creator, developer, admin
    Email    string `json:"email"`
    jwt.RegisteredClaims
}

// Token 配置
AccessTokenExpiry  = 15 * time.Minute
RefreshTokenExpiry = 7 * 24 * time.Hour
```

### 4. Agent Builder Service

处理 Agent 的创建、配置和版本管理。

```go
// Agent 配置接口
type AgentConfig struct {
    SystemPrompt  string            `json:"system_prompt"`
    Model         string            `json:"model"`         // gpt-4, claude-3, etc.
    Provider      string            `json:"provider"`      // openai, anthropic, google
    Temperature   float64           `json:"temperature"`   // 0.0 - 2.0
    MaxTokens     int               `json:"max_tokens"`    // 最大输出 token
    TopP          float64           `json:"top_p"`
    PricePerCall  decimal.Decimal   `json:"price_per_call"` // USD
    KnowledgeBase *KnowledgeConfig  `json:"knowledge_base,omitempty"`
}

type KnowledgeConfig struct {
    Enabled     bool     `json:"enabled"`
    FileIDs     []string `json:"file_ids"`
    ChunkSize   int      `json:"chunk_size"`    // 默认 1000
    ChunkOverlap int     `json:"chunk_overlap"` // 默认 200
    TopK        int      `json:"top_k"`         // 检索数量，默认 5
}
```

### 5. Payment Service

处理支付和配额管理。

```go
// 支付接口
type PaymentService interface {
    CreateStripeCheckout(userID string, amount decimal.Decimal) (*CheckoutSession, error)
    CreateCoinbaseCharge(userID string, amount decimal.Decimal) (*CoinbaseCharge, error)
    HandleStripeWebhook(payload []byte, signature string) error
    HandleCoinbaseWebhook(payload []byte, signature string) error
    GetQuota(userID string) (int64, error)
    DecrementQuota(userID string, amount int64) error
}

// 配额包定价
var QuotaPackages = []QuotaPackage{
    {Name: "Starter", Calls: 1000, PriceUSD: 10},
    {Name: "Pro", Calls: 10000, PriceUSD: 80},
    {Name: "Enterprise", Calls: 100000, PriceUSD: 500},
}
```

### 6. RAG Service

处理知识库的向量化和检索。

```go
// RAG 接口
type RAGService interface {
    ProcessDocument(agentID string, file io.Reader, filename string) error
    RetrieveContext(agentID string, query string, topK int) ([]Document, error)
    DeleteKnowledge(agentID string) error
}

// 文档处理流程
// 1. 文件上传 -> S3
// 2. 文本提取 (PDF: pdfcpu, MD: goldmark)
// 3. 文本分块 (RecursiveCharacterTextSplitter)
// 4. 生成 Embedding (OpenAI text-embedding-3-small)
// 5. 存储到向量数据库 (pgvector / Pinecone)
```

### 7. Blockchain Service

处理链上存证和结算。

```go
// 区块链接口
type BlockchainService interface {
    MintAgentToken(agentID string, creatorAddress string) (txHash string, tokenID *big.Int, error)
    GetAgentOwnership(agentID string) (*OwnershipProof, error)
    SettleEarnings(creatorAddress string, amount *big.Int) (txHash string, error)
    GetSettlementHistory(creatorAddress string) ([]Settlement, error)
}

// ERC-1155 Token Metadata
type AgentTokenMetadata struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    AgentID     string `json:"agent_id"`
    Creator     string `json:"creator"`
    CreatedAt   int64  `json:"created_at"`
    Platform    string `json:"platform"` // "AgentLink"
}
```

## Data Models

### PostgreSQL Schema

```sql
-- 用户表 (创作者和开发者共用)
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

-- 创作者资料
CREATE TABLE creator_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    display_name VARCHAR(100) NOT NULL,
    bio TEXT,
    avatar_url VARCHAR(500),
    verified BOOLEAN DEFAULT FALSE,
    total_earnings DECIMAL(18, 8) DEFAULT 0,
    pending_earnings DECIMAL(18, 8) DEFAULT 0
);

-- Agent 表
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive')),
    
    -- 配置 (加密存储)
    config_encrypted BYTEA NOT NULL,
    config_iv BYTEA NOT NULL,
    
    -- 定价
    price_per_call DECIMAL(10, 6) NOT NULL,
    
    -- 统计
    total_calls BIGINT DEFAULT 0,
    total_revenue DECIMAL(18, 8) DEFAULT 0,
    average_rating DECIMAL(3, 2) DEFAULT 0,
    review_count INT DEFAULT 0,
    
    -- 区块链
    token_id BIGINT,
    token_tx_hash VARCHAR(66),
    
    -- 版本
    version INT DEFAULT 1,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

-- Agent 版本历史
CREATE TABLE agent_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    version INT NOT NULL,
    config_encrypted BYTEA NOT NULL,
    config_iv BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(agent_id, version)
);

-- API Key 表
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(8) NOT NULL,  -- 用于显示 "ak_xxxx..."
    name VARCHAR(100),
    permissions JSONB DEFAULT '{}',
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

-- 配额表
CREATE TABLE quotas (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    total_quota BIGINT DEFAULT 0,
    used_quota BIGINT DEFAULT 0,
    free_quota BIGINT DEFAULT 100,  -- 初始免费额度
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 调用日志表 (分区表，按月分区)
CREATE TABLE call_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    api_key_id UUID NOT NULL,
    user_id UUID NOT NULL,
    
    -- 请求信息
    request_id VARCHAR(36),
    input_tokens INT,
    output_tokens INT,
    latency_ms INT,
    status VARCHAR(20),  -- success, error, timeout
    error_code VARCHAR(50),
    
    -- 计费
    cost_usd DECIMAL(10, 6),
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- 支付记录表
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_usd DECIMAL(10, 2) NOT NULL,
    quota_purchased BIGINT NOT NULL,
    payment_method VARCHAR(20) NOT NULL,  -- stripe, coinbase
    payment_id VARCHAR(255),  -- 外部支付 ID
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

-- 结算记录表
CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(18, 8) NOT NULL,
    platform_fee DECIMAL(18, 8) NOT NULL,
    net_amount DECIMAL(18, 8) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    settled_at TIMESTAMP WITH TIME ZONE
);

-- 评价表
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    user_id UUID NOT NULL REFERENCES users(id),
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    content TEXT,
    status VARCHAR(20) DEFAULT 'pending',  -- pending, approved, rejected
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(agent_id, user_id)
);

-- Webhook 配置表
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    url VARCHAR(500) NOT NULL,
    events JSONB NOT NULL,  -- ["quota.low", "call.completed"]
    secret VARCHAR(64) NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 知识库文件表
CREATE TABLE knowledge_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_size BIGINT NOT NULL,
    s3_key VARCHAR(500) NOT NULL,
    chunk_count INT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'processing',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- 索引
CREATE INDEX idx_agents_creator ON agents(creator_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_category ON agents(category);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_call_logs_agent ON call_logs(agent_id);
CREATE INDEX idx_call_logs_user ON call_logs(user_id);
CREATE INDEX idx_call_logs_created ON call_logs(created_at);
CREATE INDEX idx_reviews_agent ON reviews(agent_id);
```

### Redis 数据结构

```
# API Key 缓存
apikey:{key_hash} -> {user_id, permissions, quota}
TTL: 5 minutes

# 配额计数器
quota:{user_id} -> remaining_quota
TTL: none (持久化)

# 限速计数器 (滑动窗口)
ratelimit:{user_id}:{minute} -> call_count
TTL: 2 minutes

# Agent 配置缓存
agent:{agent_id} -> {config, status, price}
TTL: 10 minutes

# 试用次数
trial:{user_id}:{agent_id} -> remaining_trials
TTL: none
```

### Vector Database Schema (pgvector)

```sql
-- 启用 pgvector 扩展
CREATE EXTENSION IF NOT EXISTS vector;

-- 知识库向量表
CREATE TABLE knowledge_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    file_id UUID NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),  -- OpenAI text-embedding-3-small
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 向量索引 (IVFFlat)
CREATE INDEX idx_embeddings_vector ON knowledge_embeddings 
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_embeddings_agent ON knowledge_embeddings(agent_id);
```

## Error Handling

### 错误码设计

```go
// 错误码结构
type ErrorCode string

const (
    // 认证错误 (401xx)
    ErrInvalidCredentials ErrorCode = "40101"
    ErrTokenExpired       ErrorCode = "40102"
    ErrInvalidAPIKey      ErrorCode = "40103"
    
    // 授权错误 (403xx)
    ErrForbidden          ErrorCode = "40301"
    ErrAgentNotOwned      ErrorCode = "40302"
    
    // 资源错误 (404xx)
    ErrAgentNotFound      ErrorCode = "40401"
    ErrUserNotFound       ErrorCode = "40402"
    
    // 请求错误 (400xx)
    ErrInvalidRequest     ErrorCode = "40001"
    ErrValidationFailed   ErrorCode = "40002"
    
    // 限制错误 (429xx)
    ErrQuotaExhausted     ErrorCode = "42901"
    ErrRateLimited        ErrorCode = "42902"
    
    // 服务器错误 (500xx)
    ErrInternalServer     ErrorCode = "50001"
    ErrUpstreamTimeout    ErrorCode = "50401"
    ErrUpstreamUnavailable ErrorCode = "50301"
)

// 错误响应格式
type ErrorResponse struct {
    Error struct {
        Code    ErrorCode `json:"code"`
        Message string    `json:"message"`
        Details any       `json:"details,omitempty"`
    } `json:"error"`
    RequestID string `json:"request_id"`
}
```

### 熔断器配置

```go
// Circuit Breaker 配置
type CircuitBreakerConfig struct {
    MaxRequests   uint32        // 半开状态最大请求数
    Interval      time.Duration // 统计窗口
    Timeout       time.Duration // 熔断超时
    FailureRatio  float64       // 失败率阈值
}

var DefaultCircuitBreaker = CircuitBreakerConfig{
    MaxRequests:  5,
    Interval:     10 * time.Second,
    Timeout:      30 * time.Second,
    FailureRatio: 0.5,
}
```

## Frontend Architecture

### Next.js 14 App Router 结构

```
app/
├── (marketing)/              # 营销页面组
│   ├── page.tsx              # Landing Page
│   ├── pricing/page.tsx      # 定价页
│   └── about/page.tsx        # 关于页
├── (auth)/                   # 认证页面组
│   ├── login/page.tsx
│   ├── register/page.tsx
│   └── layout.tsx            # 认证布局
├── (dashboard)/              # 仪表盘组
│   ├── layout.tsx            # 侧边栏布局
│   ├── creator/              # 创作者仪表盘
│   │   ├── page.tsx          # 概览
│   │   ├── agents/page.tsx   # Agent 列表
│   │   ├── agents/[id]/page.tsx
│   │   ├── analytics/page.tsx
│   │   └── earnings/page.tsx
│   ├── developer/            # 开发者控制台
│   │   ├── page.tsx          # 概览
│   │   ├── keys/page.tsx     # API Keys
│   │   ├── usage/page.tsx    # 使用统计
│   │   └── billing/page.tsx  # 账单
│   └── admin/                # 管理后台
│       ├── page.tsx
│       ├── users/page.tsx
│       └── moderation/page.tsx
├── marketplace/              # 商城
│   ├── page.tsx              # 搜索/浏览
│   └── [agentId]/page.tsx    # Agent 详情
├── playground/               # API Playground
│   └── [agentId]/page.tsx
└── api/                      # API Routes (BFF)
    └── [...proxy]/route.ts   # 代理到 Go 后端
```

### 状态管理

```typescript
// 使用 Zustand 进行客户端状态管理
import { create } from 'zustand';

interface AuthStore {
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

// 使用 TanStack Query 进行服务端状态管理
import { useQuery, useMutation } from '@tanstack/react-query';

// Agent 列表查询
const useAgents = () => useQuery({
  queryKey: ['agents'],
  queryFn: () => api.get('/agents'),
  staleTime: 5 * 60 * 1000, // 5 分钟
});
```

### UI 组件设计

```typescript
// 基于 Shadcn UI 的组件层次
components/
├── ui/                       # Shadcn 基础组件
│   ├── button.tsx
│   ├── card.tsx
│   ├── dialog.tsx
│   └── ...
├── common/                   # 通用业务组件
│   ├── Header.tsx
│   ├── Sidebar.tsx
│   ├── DataTable.tsx
│   └── Charts.tsx
├── agent/                    # Agent 相关组件
│   ├── AgentCard.tsx
│   ├── AgentBuilder.tsx
│   ├── AgentPreview.tsx
│   └── PricingConfig.tsx
├── developer/                # 开发者相关组件
│   ├── APIKeyCard.tsx
│   ├── UsageChart.tsx
│   └── CodeSnippet.tsx
└── creator/                  # 创作者相关组件
    ├── RevenueChart.tsx
    ├── EarningsCard.tsx
    └── WithdrawalForm.tsx
```

### 主题配置

```typescript
// tailwind.config.ts
const config = {
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#f0f9ff',
          // ... 完整色阶
          900: '#0c4a6e',
        },
        // 语义化颜色
        success: '#10b981',
        warning: '#f59e0b',
        error: '#ef4444',
      },
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
    },
  },
};
```

## Security Design

### 认证与授权

```go
// JWT 配置
type JWTConfig struct {
    SecretKey          string        // 从环境变量加载
    AccessTokenExpiry  time.Duration // 15 分钟
    RefreshTokenExpiry time.Duration // 7 天
    Issuer             string        // "agentlink"
}

// 密码哈希 (Argon2id)
func HashPassword(password string) (string, error) {
    return argon2id.CreateHash(password, argon2id.DefaultParams)
}
```

### API 安全

```go
// CORS 配置
func CORSMiddleware() gin.HandlerFunc {
    return cors.New(cors.Config{
        AllowOrigins:     []string{"https://agentlink.io", "https://app.agentlink.io"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-ID"},
        ExposeHeaders:    []string{"X-Request-ID", "X-RateLimit-Remaining"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    })
}

// Rate Limiting 中间件
func RateLimitMiddleware(rdb *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        key := fmt.Sprintf("ratelimit:%s:%d", userID, time.Now().Unix()/60)
        
        count, _ := rdb.Incr(c, key).Result()
        rdb.Expire(c, key, 2*time.Minute)
        
        limit := getUserRateLimit(userID) // 10 or 1000
        if count > int64(limit) {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
            return
        }
        
        c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
        c.Header("X-RateLimit-Remaining", strconv.Itoa(limit-int(count)))
        c.Next()
    }
}
```

### 加密服务

```go
// AES-256-GCM 加密
type EncryptionService struct {
    key []byte // 32 bytes for AES-256
}

func (e *EncryptionService) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
    block, _ := aes.NewCipher(e.key)
    gcm, _ := cipher.NewGCM(block)
    
    nonce = make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    
    ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
    return
}

func (e *EncryptionService) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
    block, _ := aes.NewCipher(e.key)
    gcm, _ := cipher.NewGCM(block)
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

### 密钥管理

```yaml
# 密钥轮换策略
secrets:
  jwt_secret:
    rotation: 90 days
    storage: AWS Secrets Manager / HashiCorp Vault
  encryption_key:
    rotation: 365 days
    storage: AWS KMS
  api_keys:
    format: "ak_{random_32_chars}"
    hash: SHA-256
    storage: PostgreSQL (hashed only)
```

## Monitoring & Observability

### 日志设计

```go
// 结构化日志 (使用 zerolog)
import "github.com/rs/zerolog"

type RequestLog struct {
    RequestID   string        `json:"request_id"`
    Method      string        `json:"method"`
    Path        string        `json:"path"`
    StatusCode  int           `json:"status_code"`
    Latency     time.Duration `json:"latency_ms"`
    UserID      string        `json:"user_id,omitempty"`
    AgentID     string        `json:"agent_id,omitempty"`
    Error       string        `json:"error,omitempty"`
}

// 日志级别
// - DEBUG: 开发调试信息
// - INFO: 正常业务流程
// - WARN: 可恢复的异常
// - ERROR: 需要关注的错误
// - FATAL: 系统无法继续运行
```

### 指标收集 (Prometheus)

```go
// 核心指标
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"method", "path"},
    )
    
    aiProviderLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_provider_latency_seconds",
            Help:    "AI provider response latency",
            Buckets: []float64{.5, 1, 2, 5, 10, 20, 30},
        },
        []string{"provider", "model"},
    )
    
    quotaRemaining = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "quota_remaining",
            Help: "Remaining API quota per user",
        },
        []string{"user_id"},
    )
)
```

### 告警规则

```yaml
# Prometheus Alerting Rules
groups:
  - name: agentlink
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          
      - alert: AIProviderLatency
        expr: histogram_quantile(0.95, ai_provider_latency_seconds) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "AI provider latency is high"
          
      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state == 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker is open for {{ $labels.provider }}"
```

### 分布式追踪 (OpenTelemetry)

```go
// Trace 配置
func InitTracer() (*sdktrace.TracerProvider, error) {
    exporter, _ := otlptracehttp.New(context.Background())
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("agentlink-api"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

## Caching Strategy

### 缓存层次

```
┌─────────────────────────────────────────────────────────────┐
│                      Browser Cache                          │
│  - Static assets (JS, CSS, images): 1 year                 │
│  - API responses: no-cache (use SWR/React Query)           │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                        CDN Cache                            │
│  - Static assets: 1 year                                   │
│  - Public pages (Landing, Marketplace): 5 minutes          │
│  - API: pass-through                                       │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                      Redis Cache                            │
│  - API Key validation: 5 minutes                           │
│  - Agent config: 10 minutes                                │
│  - Rate limit counters: 2 minutes                          │
│  - Session data: 24 hours                                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                      PostgreSQL                             │
│  - Source of truth                                         │
└─────────────────────────────────────────────────────────────┘
```

### 缓存失效策略

```go
// 写穿透 (Write-Through) 模式
func UpdateAgent(ctx context.Context, agent *Agent) error {
    // 1. 更新数据库
    if err := db.Save(agent).Error; err != nil {
        return err
    }
    
    // 2. 删除缓存 (让下次读取时重建)
    cacheKey := fmt.Sprintf("agent:%s", agent.ID)
    redis.Del(ctx, cacheKey)
    
    // 3. 发布失效事件 (多实例同步)
    redis.Publish(ctx, "cache:invalidate", cacheKey)
    
    return nil
}

// 缓存预热
func WarmupCache(ctx context.Context) {
    // 预热热门 Agent 配置
    hotAgents := getHotAgents(100)
    for _, agent := range hotAgents {
        cacheAgent(ctx, agent)
    }
}
```

## Admin API

### 管理后台接口

```go
// Admin API 路由
/api/v1/admin/
├── dashboard/
│   └── GET    /stats            // 平台统计数据
├── users/
│   ├── GET    /                 // 用户列表 (分页、搜索)
│   ├── GET    /:id              // 用户详情
│   ├── PUT    /:id/status       // 更新用户状态 (suspend/activate)
│   └── DELETE /:id              // 删除用户
├── agents/
│   ├── GET    /                 // Agent 列表
│   ├── GET    /:id              // Agent 详情
│   └── PUT    /:id/status       // 更新 Agent 状态
├── moderation/
│   ├── GET    /queue            // 待审核内容队列
│   ├── POST   /reviews/:id/approve
│   └── POST   /reviews/:id/reject
├── settlements/
│   ├── GET    /pending          // 待结算列表
│   ├── POST   /trigger          // 手动触发结算
│   └── GET    /history          // 结算历史
└── system/
    ├── GET    /health           // 系统健康检查
    ├── GET    /metrics          // Prometheus 指标
    └── POST   /cache/clear      // 清除缓存
```

### 权限控制

```go
// RBAC 权限模型
type Permission string

const (
    PermViewUsers       Permission = "users:view"
    PermManageUsers     Permission = "users:manage"
    PermViewAgents      Permission = "agents:view"
    PermManageAgents    Permission = "agents:manage"
    PermModerateContent Permission = "content:moderate"
    PermManagePayments  Permission = "payments:manage"
    PermSystemAdmin     Permission = "system:admin"
)

type Role struct {
    Name        string       `json:"name"`
    Permissions []Permission `json:"permissions"`
}

var AdminRoles = map[string]Role{
    "super_admin": {
        Name: "Super Admin",
        Permissions: []Permission{
            PermViewUsers, PermManageUsers,
            PermViewAgents, PermManageAgents,
            PermModerateContent, PermManagePayments,
            PermSystemAdmin,
        },
    },
    "moderator": {
        Name: "Moderator",
        Permissions: []Permission{
            PermViewUsers, PermViewAgents,
            PermModerateContent,
        },
    },
    "support": {
        Name: "Support",
        Permissions: []Permission{
            PermViewUsers, PermViewAgents,
        },
    },
}
```

## Configuration Management

### 环境变量

```bash
# .env.example
# Database
DATABASE_URL=postgres://user:pass@localhost:5432/agentlink
DATABASE_MAX_CONNECTIONS=100

# Redis
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=

# JWT
JWT_SECRET=your-256-bit-secret
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Encryption
ENCRYPTION_KEY=your-32-byte-key

# AI Providers
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_AI_API_KEY=xxx

# Payment
STRIPE_SECRET_KEY=sk_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
COINBASE_API_KEY=xxx
COINBASE_WEBHOOK_SECRET=xxx

# Blockchain
BLOCKCHAIN_RPC_URL=https://base-mainnet.g.alchemy.com/v2/xxx
BLOCKCHAIN_PRIVATE_KEY=xxx
CONTRACT_ADDRESS=0x...

# Storage
S3_BUCKET=agentlink-files
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx

# Monitoring
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

### 特性开关 (Feature Flags)

```go
// 特性开关配置
type FeatureFlags struct {
    EnableCryptoPayments  bool `json:"enable_crypto_payments"`
    EnableBlockchainMint  bool `json:"enable_blockchain_mint"`
    EnableRAG             bool `json:"enable_rag"`
    EnableWebhooks        bool `json:"enable_webhooks"`
    EnableTrialMode       bool `json:"enable_trial_mode"`
    MaintenanceMode       bool `json:"maintenance_mode"`
}

// 从 Redis 或配置服务加载
func GetFeatureFlags(ctx context.Context) *FeatureFlags {
    // 支持运行时动态更新
}
```

## Testing Strategy

### 测试分层

1. **单元测试**: 测试独立函数和方法
2. **集成测试**: 测试服务间交互
3. **端到端测试**: 测试完整用户流程
4. **属性测试**: 验证系统不变量

### 测试框架

- **Go Backend**: `testing` + `testify` + `rapid` (property-based)
- **Next.js Frontend**: `Jest` + `React Testing Library` + `fast-check`
- **E2E**: `Playwright`

### 属性测试配置

```go
// Go property-based testing with rapid
import "pgregory.net/rapid"

func TestPropertyExample(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // 生成随机输入
        // 验证属性
    })
}
```

```typescript
// TypeScript property-based testing with fast-check
import * as fc from 'fast-check';

test('property example', () => {
    fc.assert(
        fc.property(fc.string(), (s) => {
            // 验证属性
            return true;
        }),
        { numRuns: 100 }
    );
});
```


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Prompt Security (Critical)

*For any* API call to any Agent, the response body and headers SHALL never contain the creator's original System Prompt text.

**Validates: Requirements 5.3, 10.2**

### Property 2: ID Uniqueness

*For any* number of created Agents and API Keys, all generated IDs SHALL be unique across the entire system.

**Validates: Requirements 2.3, 4.2**

### Property 3: Quota Consistency

*For any* sequence of successful API calls, the quota SHALL decrease by exactly the number of successful calls, and the number of call logs SHALL equal the number of successful calls.

**Validates: Requirements 5.5**

### Property 4: Failed Calls Don't Cost Quota

*For any* API call that fails due to upstream provider errors, the developer's quota SHALL remain unchanged before and after the call.

**Validates: Requirements 18.3**

### Property 5: Authentication Correctness

*For any* valid API key with sufficient quota, the Proxy Gateway SHALL accept the request. *For any* invalid API key, the Proxy Gateway SHALL return 401. *For any* valid API key with exhausted quota, the Proxy Gateway SHALL return 429.

**Validates: Requirements 5.1, 5.2**

### Property 6: Rate Limiting Enforcement

*For any* user, if they exceed their rate limit (10/min for free, 1000/min for paid), subsequent requests within the same minute SHALL return 429.

**Validates: Requirements 5.6**

### Property 7: Draft Agent Inaccessibility

*For any* Agent in draft status, all API calls to that Agent SHALL be rejected with appropriate error.

**Validates: Requirements 2.5**

### Property 8: Publish State Transition

*For any* Agent, publishing SHALL change status to active, and the Agent SHALL become accessible via API.

**Validates: Requirements 3.1**

### Property 9: Version Preservation

*For any* Agent update, the previous version SHALL be preserved and retrievable, and the version number SHALL increment.

**Validates: Requirements 3.2**

### Property 10: Wallet Address Validation

*For any* string input as wallet address, the system SHALL accept only valid Ethereum addresses (0x followed by 40 hex characters) and reject all others.

**Validates: Requirements 1.2**

### Property 11: Price Validation

*For any* price input, the system SHALL accept only values within the valid range (e.g., $0.001 - $100 per call) and reject values outside this range.

**Validates: Requirements 2.4**

### Property 12: API Key Revocation Immediacy

*For any* revoked API key, all subsequent requests using that key SHALL be rejected immediately.

**Validates: Requirements 4.3**

### Property 13: Payment-Quota Consistency

*For any* confirmed payment of amount X for quota package Y, the developer's quota SHALL increase by exactly Y calls.

**Validates: Requirements 6.3**

### Property 14: Failed Payment No-Credit

*For any* failed payment, the developer's quota SHALL remain unchanged.

**Validates: Requirements 6.4**

### Property 15: Settlement Calculation Accuracy

*For any* settlement period, the calculated creator earnings SHALL equal the sum of all call revenues minus the platform fee percentage.

**Validates: Requirements 8.1**

### Property 16: Withdrawal Fee Calculation

*For any* withdrawal request of amount X, the net amount transferred SHALL equal X minus the platform fee (e.g., 20%).

**Validates: Requirements 20.3**

### Property 17: Withdrawal Threshold Enforcement

*For any* withdrawal request below the minimum threshold, the system SHALL reject the request.

**Validates: Requirements 20.1**

### Property 18: RAG Retrieval Relevance

*For any* query to an Agent with knowledge base, the retrieved context chunks SHALL have cosine similarity above a threshold with the query embedding.

**Validates: Requirements 9.4**

### Property 19: Encryption Round-Trip

*For any* System Prompt, encrypting then decrypting with the correct key SHALL produce the original prompt.

**Validates: Requirements 10.1**

### Property 20: Search Filter Accuracy

*For any* search with filters (category, price range, rating), all returned Agents SHALL match all specified filter criteria.

**Validates: Requirements 14.3**

### Property 21: Rating Calculation Accuracy

*For any* Agent with reviews, the displayed average rating SHALL equal the mathematical average of all approved review ratings.

**Validates: Requirements 15.3**

### Property 22: Review Eligibility Enforcement

*For any* developer with fewer than 10 calls to an Agent, review submission SHALL be rejected.

**Validates: Requirements 15.1**

### Property 23: Duplicate Review Prevention

*For any* developer who has already reviewed an Agent, subsequent review submissions for the same Agent SHALL be rejected.

**Validates: Requirements 15.5**

### Property 24: Webhook Retry Logic

*For any* failed webhook delivery, the system SHALL retry up to 3 times with exponential backoff before marking as failed.

**Validates: Requirements 16.4**

### Property 25: Trial Quota Enforcement

*For any* developer-Agent pair, exactly 3 free trial calls SHALL be allowed before requiring paid quota.

**Validates: Requirements 22.1**

### Property 26: Circuit Breaker Behavior

*For any* upstream provider, if failure rate exceeds threshold within the interval, the circuit SHALL open and reject requests with 503.

**Validates: Requirements 18.2**

### Property 27: Timeout Enforcement

*For any* API call exceeding 30 seconds, the system SHALL return 504 Gateway Timeout.

**Validates: Requirements 18.4**

### Property 28: Multi-Version API Support

*For any* supported API version, requests to that version SHALL be processed correctly without affecting other versions.

**Validates: Requirements 23.3**

### Property 29: Deprecation Warning Headers

*For any* request to a deprecated API endpoint, the response SHALL include deprecation warning headers.

**Validates: Requirements 23.4**

### Property 30: Token Metadata Completeness

*For any* minted Agent token, the metadata SHALL contain AgentID, creator address, and creation timestamp.

**Validates: Requirements 7.2**

### Property 31: Analytics Data Accuracy

*For any* time period, the displayed call volume SHALL match the count of call logs in that period.

**Validates: Requirements 17.1**

### Property 32: Withdrawal Failure Recovery

*For any* failed withdrawal, the withdrawn amount SHALL be returned to the creator's available balance.

**Validates: Requirements 20.5**

### Property 33: Error Response Standardization

*For any* upstream error, the Proxy Gateway SHALL return a response conforming to the standard error format with appropriate error code.

**Validates: Requirements 18.1**

### Property 34: Correlation ID Presence

*For any* error response, the response SHALL include a correlation ID for tracing.

**Validates: Requirements 18.6**

### Property 35: Free Quota Initialization

*For any* newly registered developer, the account SHALL be initialized with the specified free quota amount.

**Validates: Requirements 4.1**


## Git Workflow & CI/CD

### 分支策略

```
main                    # 主分支 - 功能更新和项目推进
├── develop             # 开发分支 - 日常开发集成
├── feature/*           # 功能分支 - 新功能开发
│   ├── feature/auth-system
│   ├── feature/proxy-gateway
│   └── feature/payment-integration
├── bugfix/*            # 修复分支 - Bug 修复
│   └── bugfix/quota-calculation
└── error-fix/*         # 报错优化分支 - 遇到报错时创建
    ├── error-fix/db-connection-timeout
    └── error-fix/redis-cluster-failover
```

### 分支命名规范

| 分支类型 | 命名格式 | 示例 | 说明 |
|---------|---------|------|------|
| 主分支 | `main` | `main` | 稳定版本，功能更新 |
| 开发分支 | `develop` | `develop` | 开发集成 |
| 功能分支 | `feature/{feature-name}` | `feature/agent-builder` | 新功能开发 |
| 修复分支 | `bugfix/{issue-id}-{desc}` | `bugfix/123-fix-login` | Bug 修复 |
| 报错分支 | `error-fix/{error-type}` | `error-fix/sse-streaming` | 报错优化 |
| 发布分支 | `release/v{version}` | `release/v1.0.0` | 版本发布 |

### 提交规范 (Conventional Commits)

```bash
# 格式
<type>(<scope>): <subject>

# 类型
feat:     新功能
fix:      Bug 修复
docs:     文档更新
style:    代码格式 (不影响功能)
refactor: 重构
perf:     性能优化
test:     测试相关
chore:    构建/工具相关
error:    报错修复 (用于 error-fix 分支)

# 示例
feat(proxy): implement SSE streaming for AI responses
fix(auth): resolve JWT token refresh race condition
error(redis): fix cluster connection timeout issue
docs(api): update API documentation for v1.2
```

### 工作流程

```mermaid
gitGraph
    commit id: "init"
    branch develop
    checkout develop
    commit id: "setup"
    
    branch feature/auth
    checkout feature/auth
    commit id: "auth-1"
    commit id: "auth-2"
    checkout develop
    merge feature/auth
    
    branch error-fix/db-timeout
    checkout error-fix/db-timeout
    commit id: "fix-timeout"
    commit id: "add-retry"
    checkout develop
    merge error-fix/db-timeout
    
    checkout main
    merge develop tag: "v0.1.0"
```

### CI/CD Pipeline

```yaml
# .github/workflows/ci.yml
name: CI/CD Pipeline

on:
  push:
    branches: [main, develop, 'feature/**', 'error-fix/**']
  pull_request:
    branches: [main, develop]

jobs:
  # 代码检查
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Lint Go
        run: golangci-lint run
      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: Lint Frontend
        run: |
          cd frontend
          npm ci
          npm run lint

  # 单元测试
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: test
        ports:
          - 5432:5432
      redis:
        image: redis:7
        ports:
          - 6379:6379
    steps:
      - uses: actions/checkout@v4
      - name: Run Go Tests
        run: go test -v -race -coverprofile=coverage.out ./...
      - name: Run Frontend Tests
        run: |
          cd frontend
          npm ci
          npm test -- --coverage

  # 属性测试 (Property-Based Testing)
  property-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run Property Tests
        run: go test -v -tags=property ./... -count=1

  # 构建
  build:
    needs: [lint, test]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build Go Binary
        run: |
          CGO_ENABLED=0 go build -o bin/api ./cmd/api
          CGO_ENABLED=0 go build -o bin/proxy ./cmd/proxy
      - name: Build Frontend
        run: |
          cd frontend
          npm ci
          npm run build
      - name: Build Docker Images
        run: |
          docker build -t agentlink-api:${{ github.sha }} -f Dockerfile.api .
          docker build -t agentlink-proxy:${{ github.sha }} -f Dockerfile.proxy .
          docker build -t agentlink-frontend:${{ github.sha }} -f Dockerfile.frontend .

  # 部署到 Staging (develop 分支)
  deploy-staging:
    needs: build
    if: github.ref == 'refs/heads/develop'
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to Staging
        run: |
          # 部署到 staging 环境
          echo "Deploying to staging..."

  # 部署到 Production (main 分支)
  deploy-production:
    needs: build
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment: production
    steps:
      - name: Deploy to Production
        run: |
          # 部署到 production 环境
          echo "Deploying to production..."
```

### 报错处理流程

```mermaid
flowchart TD
    A[遇到报错] --> B{报错类型}
    B -->|编译错误| C[创建 error-fix/compile-xxx 分支]
    B -->|运行时错误| D[创建 error-fix/runtime-xxx 分支]
    B -->|配置错误| E[创建 error-fix/config-xxx 分支]
    
    C --> F[分析错误原因]
    D --> F
    E --> F
    
    F --> G[编写修复代码]
    G --> H[添加测试用例]
    H --> I[提交到报错分支]
    I --> J[创建 PR 到 develop]
    J --> K{Code Review}
    K -->|通过| L[合并到 develop]
    K -->|需修改| G
    L --> M[测试通过后合并到 main]
    
    subgraph "提交信息格式"
        N["error(scope): 错误描述

原因分析:
- 具体原因

解决方案:
- 修复步骤

测试验证:
- 验证方法"]
    end
```

### 错误文档模板

```markdown
# Error Fix: [错误标题]

## 错误信息
```
[粘贴完整错误堆栈]
```

## 环境信息
- OS: 
- Go Version: 
- Node Version: 
- 相关依赖版本:

## 原因分析
[详细分析错误产生的原因]

## 解决方案
[具体的修复步骤]

## 代码变更
[列出修改的文件和关键代码]

## 测试验证
[如何验证修复是否成功]

## 预防措施
[如何避免类似问题再次发生]
```

## Deployment Architecture

### 容器化部署

```yaml
# docker-compose.yml (开发环境)
version: '3.8'

services:
  api:
    build:
      context: .
      dockerfile: Dockerfile.api
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://postgres:postgres@db:5432/agentlink
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis

  proxy:
    build:
      context: .
      dockerfile: Dockerfile.proxy
    ports:
      - "8081:8081"
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080

  db:
    image: postgres:16
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_DB=agentlink
      - POSTGRES_PASSWORD=postgres

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

  vector-db:
    image: pgvector/pgvector:pg16
    volumes:
      - vector_data:/var/lib/postgresql/data

volumes:
  postgres_data:
  redis_data:
  vector_data:
```

### Kubernetes 部署 (生产环境)

```yaml
# k8s/deployment-api.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agentlink-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agentlink-api
  template:
    metadata:
      labels:
        app: agentlink-api
    spec:
      containers:
        - name: api
          image: agentlink-api:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          envFrom:
            - secretRef:
                name: agentlink-secrets
            - configMapRef:
                name: agentlink-config
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: agentlink-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: agentlink-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

## Project Structure

### 后端项目结构 (Go)

```
backend/
├── cmd/
│   ├── api/                  # API 服务入口
│   │   └── main.go
│   └── proxy/                # Proxy Gateway 入口
│       └── main.go
├── internal/
│   ├── config/               # 配置管理
│   ├── domain/               # 领域模型
│   │   ├── agent/
│   │   ├── user/
│   │   └── payment/
│   ├── handler/              # HTTP 处理器
│   │   ├── auth.go
│   │   ├── agent.go
│   │   └── proxy.go
│   ├── middleware/           # 中间件
│   │   ├── auth.go
│   │   ├── ratelimit.go
│   │   └── logging.go
│   ├── repository/           # 数据访问层
│   │   ├── postgres/
│   │   └── redis/
│   ├── service/              # 业务逻辑层
│   │   ├── auth/
│   │   ├── agent/
│   │   ├── payment/
│   │   ├── rag/
│   │   └── blockchain/
│   └── pkg/                  # 内部工具包
│       ├── crypto/
│       ├── validator/
│       └── errors/
├── migrations/               # 数据库迁移
├── tests/                    # 测试
│   ├── unit/
│   ├── integration/
│   └── property/
├── go.mod
├── go.sum
└── Makefile
```

### 前端项目结构 (Next.js)

```
frontend/
├── app/                      # App Router 页面
│   ├── (marketing)/
│   ├── (auth)/
│   ├── (dashboard)/
│   ├── marketplace/
│   └── api/
├── components/               # React 组件
│   ├── ui/                   # Shadcn UI 组件
│   ├── common/
│   ├── agent/
│   ├── creator/
│   └── developer/
├── lib/                      # 工具库
│   ├── api.ts                # API 客户端
│   ├── auth.ts               # 认证工具
│   └── utils.ts
├── hooks/                    # 自定义 Hooks
│   ├── useAuth.ts
│   ├── useAgents.ts
│   └── useQuota.ts
├── stores/                   # Zustand Stores
│   ├── authStore.ts
│   └── uiStore.ts
├── types/                    # TypeScript 类型
├── styles/                   # 全局样式
├── public/                   # 静态资源
├── tests/                    # 测试
│   ├── unit/
│   └── e2e/
├── next.config.js
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

## Development Milestones

### Phase 1: 核心基础 (Week 1-4)
- [ ] 项目初始化和基础架构
- [ ] 用户认证系统 (注册/登录/JWT)
- [ ] 数据库设计和迁移
- [ ] 基础 API 框架

### Phase 2: Agent 系统 (Week 5-8)
- [ ] Agent 创建和配置
- [ ] Proxy Gateway 核心逻辑
- [ ] SSE 流式响应
- [ ] 配额管理和限速

### Phase 3: 商业化 (Week 9-12)
- [ ] Stripe 支付集成
- [ ] Coinbase 加密支付
- [ ] 创作者提现功能
- [ ] 结算系统

### Phase 4: 高级功能 (Week 13-16)
- [ ] RAG 知识库集成
- [ ] 区块链存证
- [ ] Webhook 系统
- [ ] 分析仪表盘

### Phase 5: 优化与发布 (Week 17-20)
- [ ] 性能优化
- [ ] 安全审计
- [ ] 文档完善
- [ ] 正式发布
