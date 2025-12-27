# AgentLink Platform

<div align="center">

**Prompt as Asset, API as Service**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/node-20+-339933.svg)](https://nodejs.org/)

</div>

## Overview

AgentLink is a SaaS platform that enables AI creators to securely monetize their prompts and AI Agents, while allowing developers to integrate AI capabilities with a single API call.

### Key Features

- 🔒 **Secure Prompt Protection** - System prompts are encrypted and never exposed to end users
- 💰 **Flexible Monetization** - Set your own pricing, accept fiat and crypto payments
- 🔗 **Blockchain Ownership** - On-chain proof of Agent ownership via ERC-1155 tokens
- 📚 **RAG Knowledge Base** - Enhance Agents with private knowledge bases
- 🚀 **High Performance** - Go backend with Redis caching for low-latency API proxying
- 📊 **Analytics Dashboard** - Track usage, revenue, and performance metrics

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (Next.js 14)                    │
│         Landing Page │ Marketplace │ Dashboards             │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    API Layer (Go/Gin)                       │
│              API Gateway │ Proxy Gateway                    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      Core Services                          │
│   Auth │ Agent Builder │ Payment │ RAG │ Analytics          │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                       Data Layer                            │
│        PostgreSQL │ Redis │ pgvector │ S3                   │
└─────────────────────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js 14, Tailwind CSS, Shadcn UI |
| Backend | Go (Gin), PostgreSQL, Redis |
| AI | OpenAI, Anthropic Claude, Google Gemini |
| Blockchain | Base (L2), ERC-1155 |
| Payments | Stripe, Coinbase Commerce |

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker & Docker Compose
- pnpm (recommended) or npm

### 1. Clone the Repository

```bash
git clone https://github.com/aimerfeng/AgentLink.git
cd AgentLink
```

### 2. Check Development Environment

```bash
# Windows PowerShell
.\scripts\setup-dev.ps1

# macOS/Linux
chmod +x scripts/setup-dev.sh
./scripts/setup-dev.sh
```

### 3. Configure Environment

```bash
# Copy environment template
cp .env.example .env.local

# Edit .env.local with your API keys
# See docs/EXTERNAL_SERVICES.md for detailed setup instructions
```

### 4. Start Local Services

```bash
# Start PostgreSQL, Redis, MinIO, Mailhog
docker-compose up -d
```

### 5. Run Database Migrations

```bash
make migrate-up
```

### 6. Start Development Servers

```bash
# Terminal 1: Backend API
cd backend && make run-api

# Terminal 2: Proxy Gateway
cd backend && make run-proxy

# Terminal 3: Frontend
cd frontend && npm install && npm run dev
```

### 7. Access the Application

- **Frontend**: http://localhost:3000
- **API**: http://localhost:8080
- **Proxy Gateway**: http://localhost:8081
- **MinIO Console**: http://localhost:9001
- **Mailhog**: http://localhost:8025

## Project Structure

```
AgentLink/
├── backend/               # Go backend
│   ├── cmd/              # Application entry points
│   │   ├── api/          # Main API server
│   │   └── proxy/        # Proxy Gateway server
│   ├── internal/         # Private application code
│   │   ├── cache/        # Redis cache utilities
│   │   ├── config/       # Configuration management
│   │   ├── database/     # Database connection
│   │   ├── errors/       # Error handling
│   │   ├── middleware/   # HTTP middleware
│   │   ├── models/       # Data models
│   │   └── server/       # HTTP server setup
│   ├── migrations/       # Database migrations
│   ├── Makefile          # Build and development commands
│   └── go.mod            # Go module definition
├── frontend/             # Next.js frontend
│   ├── src/
│   │   ├── app/          # App Router pages
│   │   │   ├── (auth)/   # Auth pages (login, register)
│   │   │   ├── (dashboard)/ # Dashboard pages
│   │   │   └── marketplace/ # Marketplace pages
│   │   ├── components/   # React components
│   │   │   ├── common/   # Shared components
│   │   │   └── ui/       # Shadcn UI components
│   │   ├── lib/          # Utilities and API client
│   │   └── store/        # Zustand state management
│   ├── package.json      # Node dependencies
│   └── tailwind.config.ts # Tailwind configuration
├── scripts/              # Development scripts
├── docs/                 # Documentation
└── docker-compose.yml    # Local development services
```

## Environment Variables

See [.env.example](.env.example) for all available configuration options.

### Required Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |
| `JWT_SECRET` | Secret key for JWT tokens |
| `ENCRYPTION_KEY` | Key for encrypting system prompts |
| `OPENAI_API_KEY` | OpenAI API key (or other AI provider) |

### Optional Variables

| Variable | Description |
|----------|-------------|
| `STRIPE_SECRET_KEY` | Stripe API key for payments |
| `BLOCKCHAIN_RPC_URL` | Blockchain RPC endpoint |
| `S3_ENDPOINT` | S3-compatible storage endpoint |

## Development

### Running Tests

```bash
# Run all backend tests
cd backend && make test

# Run with coverage
cd backend && make test-coverage

# Run linter
cd backend && make lint
```

### Building for Production

```bash
# Build backend binaries
cd backend && make build

# Build frontend
cd frontend && npm run build
```

### Database Migrations

```bash
cd backend

# Create new migration
make migrate-create name=add_users_table

# Apply migrations
make migrate-up

# Rollback last migration
make migrate-down
```

## API Documentation

API documentation is available at `/api/docs` when running the development server.

### Quick API Reference

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "secure123"}'

# Call an Agent
curl -X POST http://localhost:8081/proxy/v1/agents/{agentId}/chat \
  -H "X-AgentLink-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `chore:` Maintenance tasks
- `refactor:` Code refactoring
- `test:` Adding tests
- `error:` Error fix (for error-fix branches)

### Branch Strategy

- `main` - Production-ready code
- `develop` - Development branch
- `feature/*` - New features
- `error-fix/*` - Bug fixes

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📧 Email: support@agentlink.io
- 📖 Documentation: [docs.agentlink.io](https://docs.agentlink.io)
- 💬 Discord: [Join our community](https://discord.gg/agentlink)

---

<div align="center">
Made with ❤️ by the AgentLink Team
</div>
