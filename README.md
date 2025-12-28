# AgentLink Platform

<div align="center">

<img src="https://img.shields.io/badge/AgentLink-AI%20Agent%20Platform-blue?style=for-the-badge&logo=robot" alt="AgentLink"/>

**Build, Orchestrate, and Monetize Autonomous AI Teams**

*Powered by Google A2A Protocol | Multi-Agent Orchestration | Enterprise Ready*

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/node-20+-339933.svg)](https://nodejs.org/)
[![Docker](https://img.shields.io/badge/docker-ready-2496ED.svg)](https://docker.com/)
[![A2A Protocol](https://img.shields.io/badge/A2A-Protocol%20v0.3-green.svg)](https://a2a-protocol.org/)

[English](#english) | [中文](#中文)

</div>

---

<a name="english"></a>
## 🇺🇸 English

### 🚀 Overview

AgentLink is an enterprise-grade SaaS platform that enables AI creators to securely monetize their prompts and AI Agents, while allowing developers to integrate AI capabilities with a single API call.

**What makes AgentLink special:**
- **Level 1/2**: Single-agent capabilities with secure prompt protection
- **Level 3**: Multi-agent orchestration powered by Google A2A Protocol
- **Enterprise Ready**: Built for scale with authentication, rate limiting, and circuit breakers

### 🌟 Key Features

| Feature | Description |
|---------|-------------|
| 🔒 **Secure Prompt Protection** | System prompts encrypted with AES-256-GCM, never exposed to end users |
| 🤖 **Multi-Agent Orchestration** | Build complex AI workflows with Google A2A Protocol support |
| 💰 **Flexible Monetization** | Accept fiat (Stripe) and crypto (Coinbase) payments |
| 🔗 **Blockchain Ownership** | On-chain proof of Agent ownership via ERC-1155 tokens |
| 📚 **RAG Knowledge Base** | Enhance Agents with private knowledge using pgvector |
| 🚀 **High Performance** | Go backend with Redis caching, rate limiting, circuit breaker |
| 🔄 **Human-in-the-Loop** | Approval nodes for human oversight in workflows |
| 📊 **Analytics Dashboard** | Track usage, revenue, and performance metrics |

### 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Frontend (Next.js 14)                            │
│      Landing │ Marketplace │ Dashboards │ Workflow Studio (React Flow)  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         API Layer (Go/Gin)                              │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │ API Gateway │  │ Proxy Gateway   │  │ Orchestrator Engine (A2A)   │  │
│  │   (8080)    │  │    (8081)       │  │  Multi-Agent Coordination   │  │
│  └─────────────┘  └─────────────────┘  └─────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Core Services                                  │
│  ┌──────┐ ┌───────┐ ┌─────────┐ ┌───────┐ ┌──────────┐ ┌────────────┐  │
│  │ Auth │ │ Agent │ │ Payment │ │ Trial │ │Settlement│ │ Withdrawal │  │
│  └──────┘ └───────┘ └─────────┘ └───────┘ └──────────┘ └────────────┘  │
│  ┌───────┐ ┌──────────┐ ┌─────────────┐ ┌───────────────────────────┐  │
│  │ Squad │ │ Workflow │ │ Execution   │ │ A2A Message Protocol      │  │
│  └───────┘ └──────────┘ └─────────────┘ └───────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           Data Layer                                    │
│  ┌────────────────────┐ ┌───────┐ ┌───────────┐ ┌────────────────────┐  │
│  │ PostgreSQL 16      │ │ Redis │ │ S3 (MinIO)│ │ Blockchain (Base)  │  │
│  │ + pgvector (1536d) │ │   7   │ │           │ │ ERC-1155           │  │
│  └────────────────────┘ └───────┘ └───────────┘ └────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

### 🔧 Google A2A Protocol Support

AgentLink implements the [Google Agent2Agent (A2A) Protocol](https://a2a-protocol.org/) for multi-agent orchestration:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    A2A Protocol Integration                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────┐    A2A Message    ┌─────────┐    A2A Message    ┌─────────┐
│  │ Agent A │ ───────────────▶ │ Agent B │ ───────────────▶ │ Agent C │
│  │(Writer) │                   │(Reviewer)│                   │(Editor) │
│  └─────────┘                   └─────────┘                   └─────────┘
│       │                             │                             │
│       └─────────────────────────────┴─────────────────────────────┘
│                                     │
│                          Shared Context (JSON)
│                                     │
│                              ┌──────┴──────┐
│                              │ Orchestrator │
│                              │   Engine     │
│                              └─────────────┘
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**A2A Features:**
- 🔄 **Agent Discovery** - Agents can discover and communicate with each other
- 📨 **Message Protocol** - Standardized message format for agent communication
- 🔐 **Secure Exchange** - Encrypted context sharing between agents
- ⏳ **Async First** - Designed for long-running tasks and human-in-the-loop
- 🎯 **Task Coordination** - Complex workflow orchestration across multiple agents

### 💻 Tech Stack

| Layer | Technology |
|-------|------------|
| **Frontend** | Next.js 14, Tailwind CSS, Shadcn UI, React Flow |
| **Backend** | Go 1.23 (Gin), PostgreSQL 16, Redis 7 |
| **AI Providers** | OpenAI, Anthropic Claude, Google Gemini |
| **Vector DB** | pgvector (1536 dimensions for embeddings) |
| **Blockchain** | Base (L2), ERC-1155 NFT |
| **Payments** | Stripe, Coinbase Commerce |
| **Protocol** | Google A2A Protocol v0.3 |
| **Testing** | rapid (Property-Based Testing) |

### 📁 Project Structure

```
AgentLink/
├── backend/                      # Go backend services
│   ├── cmd/                     # Application entry points
│   │   ├── api/                # Main API server (port 8080)
│   │   ├── proxy/              # Proxy Gateway (port 8081)
│   │   └── migrate/            # Database migration tool
│   ├── internal/               # Private application code
│   │   ├── agent/             # Agent CRUD & AES-256 encryption
│   │   ├── apikey/            # API Key management
│   │   ├── auth/              # JWT + Argon2id authentication
│   │   ├── cache/             # Redis cache utilities
│   │   ├── config/            # Configuration management
│   │   ├── database/          # PostgreSQL connection pool
│   │   ├── errors/            # Standardized error handling
│   │   ├── middleware/        # HTTP middleware (JWT, CORS, Rate Limit)
│   │   ├── models/            # Data models
│   │   ├── monitoring/        # Prometheus metrics
│   │   ├── payment/           # Stripe & Coinbase integration
│   │   ├── proxy/             # Proxy Gateway (rate limit, circuit breaker)
│   │   ├── settlement/        # Creator earnings settlement
│   │   ├── trial/             # Trial mechanism
│   │   └── withdrawal/        # Creator withdrawal
│   └── migrations/            # Database migrations (6 files)
├── frontend/                    # Next.js 14 frontend
│   └── src/
│       ├── app/               # App Router pages
│       ├── components/        # React components
│       └── lib/               # Utilities & API client
├── docs/                        # Documentation
│   ├── dev-logs/              # Development logs (20+ entries)
│   │   ├── phase-0/          # Environment setup
│   │   ├── phase-1/          # Infrastructure
│   │   ├── phase-2/          # Authentication
│   │   ├── phase-3/          # Agent & Proxy Gateway
│   │   ├── phase-4/          # Level 3 Database
│   │   ├── phase-5/          # Payment & Monetization
│   │   └── error-fixes/      # Error fix logs (8 entries)
│   ├── CHANGELOG.md           # Version history
│   └── EXTERNAL_SERVICES.md   # External services setup
├── scripts/                     # Development scripts
├── docker-compose.yml           # Local development services
└── README.md                    # This file
```

### 🚀 Quick Start

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

| Service | URL | Description |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | Web application |
| API | http://localhost:8080 | REST API |
| Proxy Gateway | http://localhost:8081 | AI Agent proxy |
| MinIO Console | http://localhost:9001 | S3 storage |
| Mailhog | http://localhost:8025 | Email testing |

### 📈 Development Progress

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 0 | ✅ Complete | Environment Setup (Git, Docker, External Services) |
| Phase 1 | ✅ Complete | Infrastructure (PostgreSQL, Redis, Logging, Monitoring) |
| Phase 2 | ✅ Complete | Authentication (JWT, Argon2id, Wallet Binding) |
| Phase 3 | ✅ Complete | Agent System & Proxy Gateway (CRUD, Encryption, Rate Limit) |
| Phase 4 | ✅ Complete | Level 3 Database Extension (Squad, Workflow, Execution) |
| Phase 5 | ✅ Complete | Payment & Monetization (Stripe, Coinbase, Settlement) |
| Phase 6 | ⏳ Pending | Advanced Features (RAG, Blockchain, Multi-AI) |
| Phase 7 | ⏳ Pending | Squad Service (Level 3 Multi-Agent) |
| Phase 8 | ⏳ Pending | Workflow Service (DAG Validation) |
| Phase 9 | ⏳ Pending | Orchestrator Engine (A2A Protocol) |
| Phase 10+ | ⏳ Pending | Advanced Execution, Human-in-the-Loop, Frontend |

### 📚 Documentation

| Document | Description |
|----------|-------------|
| [Development Logs](docs/dev-logs/) | 20+ detailed implementation logs |
| [Error Fixes](docs/dev-logs/error-fixes/) | 8 common issues with solutions |
| [External Services](docs/EXTERNAL_SERVICES.md) | Third-party service setup guide |
| [Changelog](docs/CHANGELOG.md) | Version history and updates |

### 🔐 Security Features

- **AES-256-GCM Encryption** - System prompts encrypted at rest
- **Argon2id Password Hashing** - Industry-standard password security
- **JWT Authentication** - Secure token-based auth with refresh tokens
- **Rate Limiting** - Redis sliding window (10/1000 calls per minute)
- **Circuit Breaker** - Prevent cascade failures with sony/gobreaker
- **CORS Protection** - Configurable cross-origin resource sharing
- **API Key Management** - SHA-256 hashed keys with revocation

---

<a name="中文"></a>
## 🇨🇳 中文

### 🚀 概述

AgentLink 是一个企业级 SaaS 平台，使 AI 创作者能够安全地将其提示词和 AI Agent 变现，同时允许开发者通过单个 API 调用集成 AI 能力。

**AgentLink 的独特之处：**
- **Level 1/2**：单智能体能力，具有安全的提示词保护
- **Level 3**：基于 Google A2A 协议的多智能体编排
- **企业就绪**：内置认证、限速和熔断器，为规模化而生

### 🌟 核心特性

| 特性 | 描述 |
|------|------|
| 🔒 **安全的提示词保护** | System Prompt 使用 AES-256-GCM 加密，永不暴露给终端用户 |
| 🤖 **多智能体编排** | 支持 Google A2A 协议构建复杂的 AI 工作流 |
| 💰 **灵活的变现方式** | 支持法币（Stripe）和加密货币（Coinbase）支付 |
| 🔗 **区块链所有权** | 通过 ERC-1155 代币实现 Agent 所有权的链上证明 |
| 📚 **RAG 知识库** | 使用 pgvector 增强 Agent 的私有知识库 |
| 🚀 **高性能** | Go 后端配合 Redis 缓存、限速和熔断器 |
| 🔄 **人机协作** | 工作流中的审批节点支持人工监督 |
| 📊 **分析仪表盘** | 追踪使用量、收入和性能指标 |

### 🔧 Google A2A 协议支持

AgentLink 实现了 [Google Agent2Agent (A2A) 协议](https://a2a-protocol.org/)，用于多智能体编排：

**A2A 协议特性：**
- 🔄 **智能体发现** - 智能体可以相互发现和通信
- 📨 **消息协议** - 标准化的智能体通信消息格式
- 🔐 **安全交换** - 智能体之间的加密上下文共享
- ⏳ **异步优先** - 为长时间运行的任务和人机协作设计
- 🎯 **任务协调** - 跨多个智能体的复杂工作流编排

**A2A 协议由 Google 于 2025 年 4 月发布**，得到了 50 多家技术合作伙伴的支持，包括 Atlassian、Box、Salesforce、SAP、ServiceNow 等。该协议旨在打破不同 AI 智能体框架和供应商之间的壁垒，实现安全高效的跨平台协作。

### 💻 技术栈

| 层级 | 技术 |
|------|------|
| **前端** | Next.js 14, Tailwind CSS, Shadcn UI, React Flow |
| **后端** | Go 1.23 (Gin), PostgreSQL 16, Redis 7 |
| **AI 提供商** | OpenAI, Anthropic Claude, Google Gemini |
| **向量数据库** | pgvector (1536 维嵌入向量) |
| **区块链** | Base (L2), ERC-1155 NFT |
| **支付** | Stripe, Coinbase Commerce |
| **协议** | Google A2A Protocol v0.3 |
| **测试** | rapid (属性测试) |

### 🚀 快速开始

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

| 服务 | 地址 | 说明 |
|------|------|------|
| 前端 | http://localhost:3000 | Web 应用 |
| API | http://localhost:8080 | REST API |
| 代理网关 | http://localhost:8081 | AI Agent 代理 |
| MinIO 控制台 | http://localhost:9001 | S3 存储 |
| Mailhog | http://localhost:8025 | 邮件测试 |

### 📈 开发进度

| 阶段 | 状态 | 说明 |
|------|------|------|
| Phase 0 | ✅ 完成 | 环境准备（Git、Docker、外部服务） |
| Phase 1 | ✅ 完成 | 基础架构（PostgreSQL、Redis、日志、监控） |
| Phase 2 | ✅ 完成 | 认证系统（JWT、Argon2id、钱包绑定） |
| Phase 3 | ✅ 完成 | Agent 系统和代理网关（CRUD、加密、限速） |
| Phase 4 | ✅ 完成 | Level 3 数据库扩展（Squad、Workflow、Execution） |
| Phase 5 | ✅ 完成 | 支付与商业化（Stripe、Coinbase、结算） |
| Phase 6 | ⏳ 待开始 | 高级功能（RAG、区块链、多 AI） |
| Phase 7 | ⏳ 待开始 | Squad 服务（Level 3 多智能体） |
| Phase 8 | ⏳ 待开始 | Workflow 服务（DAG 验证） |
| Phase 9 | ⏳ 待开始 | 编排引擎（A2A 协议） |
| Phase 10+ | ⏳ 待开始 | 高级执行、人机协作、前端 |

### 📚 文档

| 文档 | 说明 |
|------|------|
| [开发日志](docs/dev-logs/) | 20+ 条详细的实现日志 |
| [错误修复](docs/dev-logs/error-fixes/) | 8 个常见问题及解决方案 |
| [外部服务](docs/EXTERNAL_SERVICES.md) | 第三方服务配置指南 |
| [变更日志](docs/CHANGELOG.md) | 版本历史和更新 |

### 🔐 安全特性

- **AES-256-GCM 加密** - System Prompt 静态加密存储
- **Argon2id 密码哈希** - 行业标准的密码安全
- **JWT 认证** - 安全的基于令牌的认证，支持刷新令牌
- **限速** - Redis 滑动窗口（每分钟 10/1000 次调用）
- **熔断器** - 使用 sony/gobreaker 防止级联故障
- **CORS 保护** - 可配置的跨域资源共享
- **API Key 管理** - SHA-256 哈希密钥，支持撤销

---

## 🤝 Contributing / 贡献

1. Fork the repository / Fork 仓库
2. Create your feature branch / 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. Commit your changes / 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch / 推送分支 (`git push origin feature/amazing-feature`)
5. Open a Pull Request / 创建 Pull Request

### Commit Convention / 提交规范

| Type | Description |
|------|-------------|
| `feat:` | New feature / 新功能 |
| `fix:` | Bug fix / 错误修复 |
| `docs:` | Documentation / 文档 |
| `chore:` | Maintenance / 维护 |
| `refactor:` | Refactoring / 重构 |
| `test:` | Tests / 测试 |
| `error:` | Error fix / 错误修复 |

### Branch Strategy / 分支策略

- `main` - Production-ready code / 生产就绪代码
- `develop` - Development branch / 开发分支
- `feature/*` - New features / 新功能
- `error-fix/*` - Bug fixes / 错误修复

## 📄 License / 许可证

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🔗 Links / 链接

- **GitHub**: https://github.com/aimerfeng/AgentLink
- **A2A Protocol**: https://a2a-protocol.org/
- **Google A2A Announcement**: https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/

---

<div align="center">

**Made with ❤️ by the AgentLink Team**

*Building the future of AI Agent collaboration*

</div>
