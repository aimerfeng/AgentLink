# =========================
# AgentLink Development Environment Setup Script (Windows PowerShell)
# =========================

Write-Host "==========================" -ForegroundColor Cyan
Write-Host "AgentLink Dev Environment Setup" -ForegroundColor Cyan
Write-Host "==========================" -ForegroundColor Cyan
Write-Host ""

# Check Go
Write-Host "Checking Go..." -ForegroundColor Yellow
try {
    $goVersion = go version 2>$null
    if ($goVersion -match "go(\d+\.\d+)") {
        $version = [version]$matches[1]
        if ($version -ge [version]"1.22") {
            Write-Host "✓ Go $($matches[1]) installed" -ForegroundColor Green
        } else {
            Write-Host "✗ Go version $($matches[1]) is too old. Please install Go 1.22+" -ForegroundColor Red
            Write-Host "  Download from: https://go.dev/dl/" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "✗ Go is not installed" -ForegroundColor Red
    Write-Host "  Download from: https://go.dev/dl/" -ForegroundColor Gray
    Write-Host "  Or install via winget: winget install GoLang.Go" -ForegroundColor Gray
}

Write-Host ""

# Check Node.js
Write-Host "Checking Node.js..." -ForegroundColor Yellow
try {
    $nodeVersion = node --version 2>$null
    if ($nodeVersion -match "v(\d+)") {
        $majorVersion = [int]$matches[1]
        if ($majorVersion -ge 20) {
            Write-Host "✓ Node.js $nodeVersion installed" -ForegroundColor Green
        } else {
            Write-Host "✗ Node.js version $nodeVersion is too old. Please install Node.js 20+" -ForegroundColor Red
            Write-Host "  Download from: https://nodejs.org/" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "✗ Node.js is not installed" -ForegroundColor Red
    Write-Host "  Download from: https://nodejs.org/" -ForegroundColor Gray
    Write-Host "  Or install via winget: winget install OpenJS.NodeJS.LTS" -ForegroundColor Gray
}

Write-Host ""

# Check pnpm
Write-Host "Checking pnpm..." -ForegroundColor Yellow
try {
    $pnpmVersion = pnpm --version 2>$null
    Write-Host "✓ pnpm $pnpmVersion installed" -ForegroundColor Green
} catch {
    Write-Host "✗ pnpm is not installed" -ForegroundColor Red
    Write-Host "  Install via: npm install -g pnpm" -ForegroundColor Gray
}

Write-Host ""

# Check Docker
Write-Host "Checking Docker..." -ForegroundColor Yellow
try {
    $dockerVersion = docker --version 2>$null
    Write-Host "✓ $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Docker is not installed" -ForegroundColor Red
    Write-Host "  Download from: https://www.docker.com/products/docker-desktop/" -ForegroundColor Gray
}

Write-Host ""

# Check Docker Compose
Write-Host "Checking Docker Compose..." -ForegroundColor Yellow
try {
    $composeVersion = docker-compose --version 2>$null
    Write-Host "✓ $composeVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Docker Compose is not installed" -ForegroundColor Red
    Write-Host "  Usually included with Docker Desktop" -ForegroundColor Gray
}

Write-Host ""

# Check golangci-lint
Write-Host "Checking golangci-lint..." -ForegroundColor Yellow
try {
    $lintVersion = golangci-lint --version 2>$null
    Write-Host "✓ $lintVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ golangci-lint is not installed" -ForegroundColor Red
    Write-Host "  Install via: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" -ForegroundColor Gray
    Write-Host "  Or download from: https://golangci-lint.run/usage/install/" -ForegroundColor Gray
}

Write-Host ""
Write-Host "==========================" -ForegroundColor Cyan
Write-Host "Setup Complete" -ForegroundColor Cyan
Write-Host "==========================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Install any missing dependencies listed above" -ForegroundColor White
Write-Host "2. Copy .env.example to .env.local and configure your API keys" -ForegroundColor White
Write-Host "3. Run 'docker-compose up -d' to start local services" -ForegroundColor White
Write-Host "4. Run 'make dev' to start the development servers" -ForegroundColor White
