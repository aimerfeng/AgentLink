# AgentLink Platform

<div align="center">

**Build, Orchestrate, and Monetize Autonomous AI Teams**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/node-20+-339933.svg)](https://nodejs.org/)
[![Docker](https://img.shields.io/badge/docker-ready-2496ED.svg)](https://docker.com/)

[English](#english) | [中文](#中文)

</div>

---

<a name="english"></a>
## 🇺🇸 English

### Overview

AgentLink is a SaaS platform that enables AI creators to securely monetize their prompts and AI Agents, while allowing developers to integrate AI capabilities with a single API call. The platform supports both single-agent (Level 1/2) and multi-agent orchestration (Level 3) capabilities.

### Key Features

- 🔒 **Secure Prompt Protection** - System prompts are AES-256-GCM encrypted and never exposed
- 💰 **Flexible Monetization** - Set your own pricing, accept fiat (Stripe) and crypto (Coinbase) payments
- 🔗 **Blockchain Ownership** - On-chain proof of Agent ownership via ERC-1155 tokens
- 📚 **RAG Knowledge Base** - Enhance Agents with private knowledge bases using pgvector
- 🚀 **High Performance** - Go backend with Redis caching, rate limiting, and circuit breaker
- 📊 **Analytics Dashboard** - Track usage, revenue, and performance metrics
- 🤖 **Multi-Agent Orchestration** - Build and execute complex AI workflows (Level 3)
- 🔄 **Human-in-the-Loop** - Approval nodes for human oversight in workflows

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Frontend (Next.js 14)                          │
│         Landing │ Marketplace │ Dashboards │ Workflow Studio        │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                       API Layer (Go/Gin)                            │
│    API Gateway (8080) │ Proxy Gateway (8081) │ Orchestrator Engine  │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                        Core Services                                │
│  Auth │ Agent │ Payment │ Trial │ Settlement │ Withdrawal │ Squad   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                         Data Layer                                  │
│      PostgreSQL (pgvector) │ Redis │ S3 (MinIO) │ Blockchain        │
└─────────────────────────────────────────────────────────────────────┘
```

### Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js 14, Tailwind CSS, Shadcn UI, React Flow |
| Backend | Go 1.23 (Gin), PostgreSQL 16, Redis 7 |
| AI | OpenAI, Anthropic Claude, Google Gemini |
| Vector DB | pgvector (1536 dimensions) |
| Blockchain | Base (L2), ERC-1155 |
| Payments | Stripe, Coinbase Commerce |
| Testing | rapid (Property-Based Testing) |

### Project Structure

```
AgentLink/
├── backend/                    # Go backend
│   ├── cmd/                   # Application entry points
│   │   ├── api/              # Main API server (port 8080)
│   │   ├── proxy/            # Proxy Gateway (port 8081)
│   │   └── migrate/          # Database migration tool
│   ├── internal/             # Private application code
│   │   ├── agent/           # Agent CRUD & encryption
│   │   ├── apikey/          # API Key management
│   │   ├── auth/            # Authentication (JWT, Argon2id)
│   │   ├── cache/           # Redis cache utilities
│   │   ├── config/          # Configuration management
│   │   ├── database/        # PostgreSQL connection
│   │   ├── errors/          # Standardized error handling
│   │   ├── middleware/      # HTTP middleware (JWT, CORS, Rate Limit)
│   │   ├── models/          # Data models
│   │   ├── monitoring/      # Prometheus metrics
│   │   ├── payment/         # Stripe & Coinbase integration
│   │   ├── proxy/           # Proxy Gateway core
│   │   ├── settlement/      # Creator earnings settlement
│   │   ├── trial/           # Trial mechanism
│   │   └── withdrawal/      # Creator withdrawal
│   └── migrations/          # Database migrations (6 files)
├── frontend/                  # Next.js frontend
│   └── src/
│       ├── app/             # App Router pages
│       ├── components/      # React components
│       └── lib/             # Utilities
├── docs/                      # Documentation
│   ├── dev-logs/            # Development logs (20+ entries)
│   │   ├── phase-0/        # Environment setup
│   │   ├── phase-1/        # Infrastructure
│   │   ├── phase-2/        # Authentication
│   │   ├── phase-3/        # Agent & Proxy Gateway
│   │   ├── phase-5/        # Payment & Monetization
│   │   └── error-fixes/    # Error fix logs
│   ├── CHANGELOG.md         # Change log
│   └── EXTERNAL_SERVICES.md # External services setup
├── scripts/                   # Development scripts
├── docker-compose.yml         # Local development services
└── README.md                  # This file
```

### Quick Start

#### Prerequisites

- Go 1.23+
- Node.js 20+
- Docker & Docker Compose
- pnpm (recommended) or npm

#### 1. Clone the Repository

```bash
git clone https://github.com/aimerfeng/AgentLink.git
cd AgentLink
```

#### 2. Configure Environment

```bash
cp .env.example .env.local
# Edit .env.local with your API keys
```

#### 3. Start Local Services

```bash
docker-compose up -d
```

#### 4. Run Database Migrations

```bash
cd backend && make migrate-up
```

#### 5. Start Development Servers

```bash
# Terminal 1: Backend API
cd backend && make run-api

# Terminal 2: Proxy Gateway
cd backend && make run-proxy

# Terminal 3: Frontend
cd frontend && npm install && npm run dev
```

#### 6. Access the Application

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |
| Proxy Gateway | http://localhost:8081 |
| MinIO Console | http://localhost:9001 |
| Mailhog | http://localhost:8025 |

### Development Progress

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 0 | ✅ Complete | Environment Setup |
| Phase 1 | ✅ Complete | Infrastructure (DB, Redis, Logging) |
| Phase 2 | ✅ Complete | Authentication (JWT, Wallet) |
| Phase 3 | ✅ Complete | Agent System & Proxy Gateway |
| Phase 4 | ✅ Complete | Level 3 Database Extension |
| Phase 5 | ✅ Complete | Payment & Monetization |
| Phase 6 | ⏳ Pending | Advanced Features (RAG, Blockchain) |
| Phase 7+ | ⏳ Pending | Multi-Agent Orchestration |

### Documentation

- [Development Logs](docs/dev-logs/) - Detailed implementation logs
- [Error Fixes](docs/dev-logs/error-fixes/) - Common issues and solutions
- [External Services](docs/EXTERNAL_SERVICES.md) - Third-party service setup
- [Changelog](docs/CHANGELOG.md) - Version history

---

<a name="中文"></a>
## 🇨🇳 中文

### 概述

AgentLink 是一个 SaaS 平台，使 AI 创作者能够安全地将其提示词和 AI Agent 变现，同时允许开发者通过单个 API 调用集成 AI 能力。平台支持单智能体（Level 1/2）和多智能体编排（Level 3）功能。

### 核心特性

- 🔒 **安全的提示词保护** - System Prompt 使用 AES-256-GCM 加密，永不暴露
- 💰 **灵活的变现方式** - 自定义定价，支持法币（Stripe）和加密货币（Coinbase）支付
- 🔗 **区块链所有权** - 通过 ERC-1155 代币实现 Agent 所有权的链上证明
- 📚 **RAG 知识库** - 使用 pgvector 增强 Agent 的私有知识库
- 🚀 **高性能** - Go 后端配合 Redis 缓存、限速和熔断器
- 📊 **分析仪表盘** - 追踪使用量、收入和性能指标
- 🤖 **多智能体编排** - 构建和执行复杂的 AI 工作流（Level 3）
- 🔄 **人机协作** - 工作流中的审批节点支持人工监督

### 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                      前端 (Next.js 14)                              │
│         落地页 │ 商城 │ 仪表盘 │ Workflow Studio                    │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                       API 层 (Go/Gin)                               │
│    API 网关 (8080) │ 代理网关 (8081) │ 编排引擎                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                        核心服务                                     │
│  认证 │ Agent │ 支付 │ 试用 │ 结算 │ 提现 │ Squad                   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                         数据层                                      │
│      PostgreSQL (pgvector) │ Redis │ S3 (MinIO) │ 区块链           │
└─────────────────────────────────────────────────────────────────────┘
```

### 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | Next.js 14, Tailwind CSS, Shadcn UI, React Flow |
| 后端 | Go 1.23 (Gin), PostgreSQL 16, Redis 7 |
| AI | OpenAI, Anthropic Claude, Google Gemini |
| 向量数据库 | pgvector (1536 维) |
| 区块链 | Base (L2), ERC-1155 |
| 支付 | Stripe, Coinbase Commerce |
| 测试 | rapid (属性测试) |

### 项目结构

```
AgentLink/
├── backend/                    # Go 后端
│   ├── cmd/                   # 应用入口
│   │   ├── api/              # 主 API 服务器 (端口 8080)
│   │   ├── proxy/            # 代理网关 (端口 8081)
│   │   └── migrate/          # 数据库迁移工具
│   ├── internal/             # 私有应用代码
│   │   ├── agent/           # Agent CRUD 和加密
│   │   ├── apikey/          # API Key 管理
│   │   ├── auth/            # 认证 (JWT, Argon2id)
│   │   ├── cache/           # Redis 缓存工具
│   │   ├── config/          # 配置管理
│   │   ├── database/        # PostgreSQL 连接
│   │   ├── errors/          # 标准化错误处理
│   │   ├── middleware/      # HTTP 中间件
│   │   ├── models/          # 数据模型
│   │   ├── monitoring/      # Prometheus 指标
│   │   ├── payment/         # Stripe 和 Coinbase 集成
│   │   ├── proxy/           # 代理网关核心
│   │   ├── settlement/      # 创作者收益结算
│   │   ├── trial/           # 试用机制
│   │   └── withdrawal/      # 创作者提现
│   └── migrations/          # 数据库迁移 (6 个文件)
├── frontend/                  # Next.js 前端
├── docs/                      # 文档
│   ├── dev-logs/            # 开发日志 (20+ 条目)
│   │   ├── phase-0/        # 环境准备
│   │   ├── phase-1/        # 基础架构
│   │   ├── phase-2/        # 认证系统
│   │   ├── phase-3/        # Agent 和代理网关
│   │   ├── phase-5/        # 支付与商业化
│   │   └── error-fixes/    # 错误修复日志
│   ├── CHANGELOG.md         # 变更日志
│   └── EXTERNAL_SERVICES.md # 外部服务配置
├── scripts/                   # 开发脚本
├── docker-compose.yml         # 本地开发服务
└── README.md                  # 本文件
```

### 快速开始

#### 前置条件

- Go 1.23+
- Node.js 20+
- Docker & Docker Compose
- pnpm（推荐）或 npm

#### 1. 克隆仓库

```bash
git clone https://github.com/aimerfeng/AgentLink.git
cd AgentLink
```

#### 2. 配置环境

```bash
cp .env.example .env.local
# 编辑 .env.local 填入你的 API 密钥
```

#### 3. 启动本地服务

```bash
docker-compose up -d
```

#### 4. 运行数据库迁移

```bash
cd backend && make migrate-up
```

#### 5. 启动开发服务器

```bash
# 终端 1: 后端 API
cd backend && make run-api

# 终端 2: 代理网关
cd backend && make run-proxy

# 终端 3: 前端
cd frontend && npm install && npm run dev
```

#### 6. 访问应用

| 服务 | 地址 |
|------|------|
| 前端 | http://localhost:3000 |
| API | http://localhost:8080 |
| 代理网关 | http://localhost:8081 |
| MinIO 控制台 | http://localhost:9001 |
| Mailhog | http://localhost:8025 |

### 开发进度

| 阶段 | 状态 | 说明 |
|------|------|------|
| Phase 0 | ✅ 完成 | 环境准备 |
| Phase 1 | ✅ 完成 | 基础架构（数据库、Redis、日志） |
| Phase 2 | ✅ 完成 | 认证系统（JWT、钱包） |
| Phase 3 | ✅ 完成 | Agent 系统和代理网关 |
| Phase 4 | ✅ 完成 | Level 3 数据库扩展 |
| Phase 5 | ✅ 完成 | 支付与商业化 |
| Phase 6 | ⏳ 待开始 | 高级功能（RAG、区块链） |
| Phase 7+ | ⏳ 待开始 | 多智能体编排 |

### 文档

- [开发日志](docs/dev-logs/) - 详细的实现日志
- [错误修复](docs/dev-logs/error-fixes/) - 常见问题和解决方案
- [外部服务](docs/EXTERNAL_SERVICES.md) - 第三方服务配置
- [变更日志](docs/CHANGELOG.md) - 版本历史

---

## Contributing / 贡献

1. Fork the repository / Fork 仓库
2. Create your feature branch / 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. Commit your changes / 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch / 推送分支 (`git push origin feature/amazing-feature`)
5. Open a Pull Request / 创建 Pull Request

### Commit Convention / 提交规范

- `feat:` New feature / 新功能
- `fix:` Bug fix / 错误修复
- `docs:` Documentation / 文档
- `chore:` Maintenance / 维护
- `refactor:` Refactoring / 重构
- `test:` Tests / 测试
- `error:` Error fix / 错误修复

### Branch Strategy / 分支策略

- `main` - Production-ready code / 生产就绪代码
- `develop` - Development branch / 开发分支
- `feature/*` - New features / 新功能
- `error-fix/*` - Bug fixes / 错误修复

## License / 许可证

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

---

<div align="center">
Made with ❤️ by the AgentLink Team
</div>
