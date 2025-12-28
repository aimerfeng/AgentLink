# 开发日志 002: 环境配置文件

**日期**: 2024-12-27  
**任务**: Task 0.2 - 创建环境配置文件  
**状态**: ✅ 已完成

## 任务描述

创建开发环境所需的配置文件模板，包括环境变量、Docker 配置等。

## 实现内容

### 1. 环境变量模板 (.env.example)

```bash
# ===========================================
# AgentLink Environment Configuration
# ===========================================

# Database
DATABASE_URL=postgres://agentlink:agentlink@localhost:5432/agentlink?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# JWT Configuration
JWT_SECRET=your-256-bit-secret-key-here-change-in-production
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Encryption (32 bytes for AES-256)
ENCRYPTION_KEY=your-32-byte-encryption-key-here

# AI Providers
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_AI_API_KEY=xxx

# Payment - Stripe
STRIPE_SECRET_KEY=sk_test_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
STRIPE_PUBLISHABLE_KEY=pk_test_xxx

# Payment - Coinbase
COINBASE_API_KEY=xxx
COINBASE_WEBHOOK_SECRET=xxx

# Blockchain (Optional)
BLOCKCHAIN_RPC_URL=https://base-sepolia.g.alchemy.com/v2/xxx
BLOCKCHAIN_PRIVATE_KEY=xxx
CONTRACT_ADDRESS=0x...

# Storage (S3 Compatible)
S3_BUCKET=agentlink-files
S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin

# Email Service
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=
SMTP_PASSWORD=
FROM_EMAIL=noreply@agentlink.io

# Server Configuration
API_PORT=8080
PROXY_PORT=8081
FRONTEND_URL=http://localhost:3000

# Environment
GO_ENV=development
NODE_ENV=development
```

### 2. Docker Compose 配置

```yaml
version: '3.8'

services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: agentlink
      POSTGRES_PASSWORD: agentlink
      POSTGRES_DB: agentlink
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

  mailhog:
    image: mailhog/mailhog
    ports:
      - "1025:1025"
      - "8025:8025"

volumes:
  postgres_data:
  redis_data:
  minio_data:
```

## 遇到的问题

### 问题 1: pgvector 镜像拉取失败

**描述**: Docker 拉取 pgvector 镜像时超时

**错误信息**:
```
Error response from daemon: Get "https://registry-1.docker.io/v2/": 
net/http: request canceled while waiting for connection
```

**解决方案**:
1. 配置 Docker 镜像加速器
2. 或使用代理

```json
// Docker Desktop Settings -> Docker Engine
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com"
  ]
}
```

### 问题 2: 端口冲突

**描述**: PostgreSQL 5432 端口被本地安装的 PostgreSQL 占用

**解决方案**:
1. 停止本地 PostgreSQL 服务
2. 或修改 Docker 映射端口

```yaml
ports:
  - "5433:5432"  # 使用 5433 端口
```

## 验证结果

- [x] .env.example 创建成功
- [x] docker-compose.yml 配置正确
- [x] 所有服务可正常启动
- [x] 服务间网络连接正常

## 相关文件

- `.env.example`
- `.env.local` (本地配置，不提交)
- `docker-compose.yml`
- `scripts/init-db.sql`
