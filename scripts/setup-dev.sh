#!/bin/bash
# =========================
# AgentLink Development Environment Setup Script (Unix/macOS/Linux)
# =========================

set -e

echo "=========================="
echo "AgentLink Dev Environment Setup"
echo "=========================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Check Go
echo -e "${YELLOW}Checking Go...${NC}"
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    if [ "$(printf '%s\n' "1.22" "$GO_VERSION" | sort -V | head -n1)" = "1.22" ]; then
        echo -e "${GREEN}✓ Go $GO_VERSION installed${NC}"
    else
        echo -e "${RED}✗ Go version $GO_VERSION is too old. Please install Go 1.22+${NC}"
        echo "  Download from: https://go.dev/dl/"
    fi
else
    echo -e "${RED}✗ Go is not installed${NC}"
    echo "  Download from: https://go.dev/dl/"
    echo "  Or install via brew: brew install go"
fi

echo ""

# Check Node.js
echo -e "${YELLOW}Checking Node.js...${NC}"
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version | grep -oP 'v\K[0-9]+')
    if [ "$NODE_VERSION" -ge 20 ]; then
        echo -e "${GREEN}✓ Node.js $(node --version) installed${NC}"
    else
        echo -e "${RED}✗ Node.js version $(node --version) is too old. Please install Node.js 20+${NC}"
        echo "  Download from: https://nodejs.org/"
    fi
else
    echo -e "${RED}✗ Node.js is not installed${NC}"
    echo "  Download from: https://nodejs.org/"
    echo "  Or install via brew: brew install node"
fi

echo ""

# Check pnpm
echo -e "${YELLOW}Checking pnpm...${NC}"
if command -v pnpm &> /dev/null; then
    echo -e "${GREEN}✓ pnpm $(pnpm --version) installed${NC}"
else
    echo -e "${RED}✗ pnpm is not installed${NC}"
    echo "  Install via: npm install -g pnpm"
fi

echo ""

# Check Docker
echo -e "${YELLOW}Checking Docker...${NC}"
if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓ $(docker --version)${NC}"
else
    echo -e "${RED}✗ Docker is not installed${NC}"
    echo "  Download from: https://www.docker.com/products/docker-desktop/"
fi

echo ""

# Check Docker Compose
echo -e "${YELLOW}Checking Docker Compose...${NC}"
if command -v docker-compose &> /dev/null; then
    echo -e "${GREEN}✓ $(docker-compose --version)${NC}"
elif docker compose version &> /dev/null; then
    echo -e "${GREEN}✓ $(docker compose version)${NC}"
else
    echo -e "${RED}✗ Docker Compose is not installed${NC}"
    echo "  Usually included with Docker Desktop"
fi

echo ""

# Check golangci-lint
echo -e "${YELLOW}Checking golangci-lint...${NC}"
if command -v golangci-lint &> /dev/null; then
    echo -e "${GREEN}✓ $(golangci-lint --version | head -n1)${NC}"
else
    echo -e "${RED}✗ golangci-lint is not installed${NC}"
    echo "  Install via: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    echo "  Or via brew: brew install golangci-lint"
fi

echo ""
echo -e "${CYAN}==========================${NC}"
echo -e "${CYAN}Setup Complete${NC}"
echo -e "${CYAN}==========================${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Install any missing dependencies listed above"
echo "2. Copy .env.example to .env.local and configure your API keys"
echo "3. Run 'docker-compose up -d' to start local services"
echo "4. Run 'make dev' to start the development servers"
