.PHONY: all build build-api build-proxy run run-api run-proxy test lint clean migrate docker-up docker-down help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
API_BINARY=agentlink-api
PROXY_BINARY=agentlink-proxy

# Build directories
BUILD_DIR=bin

# Default target
all: lint test build

## Build targets
build: build-api build-proxy ## Build all binaries

build-api: ## Build API server
	@echo "Building API server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api

build-proxy: ## Build Proxy Gateway
	@echo "Building Proxy Gateway..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(PROXY_BINARY) ./cmd/proxy

## Run targets
run-api: ## Run API server
	@echo "Starting API server..."
	$(GOCMD) run ./cmd/api

run-proxy: ## Run Proxy Gateway
	@echo "Starting Proxy Gateway..."
	$(GOCMD) run ./cmd/proxy

## Test targets
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Lint targets
lint: ## Run linter
	@echo "Running linter..."
	$(GOLINT) run ./...

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	$(GOLINT) run --fix ./...

## Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

## Database migrations
migrate-up: ## Run database migrations
	@echo "Running migrations..."
	$(GOCMD) run ./cmd/migrate -command up -dir ./migrations

migrate-down: ## Rollback database migrations (1 step)
	@echo "Rolling back migrations..."
	$(GOCMD) run ./cmd/migrate -command down -steps 1 -dir ./migrations

migrate-down-all: ## Rollback all database migrations
	@echo "Rolling back all migrations..."
	$(GOCMD) run ./cmd/migrate -command down -dir ./migrations

migrate-version: ## Show current migration version
	@echo "Checking migration version..."
	$(GOCMD) run ./cmd/migrate -command version -dir ./migrations

migrate-force: ## Force migration version (usage: make migrate-force version=1)
	@echo "Forcing migration version..."
	$(GOCMD) run ./cmd/migrate -command force -steps $(version) -dir ./migrations

migrate-create: ## Create a new migration (usage: make migrate-create name=migration_name)
	@echo "Creating migration..."
	@if [ -z "$(name)" ]; then echo "Error: name is required. Usage: make migrate-create name=migration_name"; exit 1; fi
	@NEXT_NUM=$$(ls -1 ./migrations/*.up.sql 2>/dev/null | wc -l | xargs); \
	NEXT_NUM=$$(printf "%06d" $$((NEXT_NUM + 1))); \
	touch "./migrations/$${NEXT_NUM}_$(name).up.sql"; \
	touch "./migrations/$${NEXT_NUM}_$(name).down.sql"; \
	echo "Created migrations/$${NEXT_NUM}_$(name).up.sql and migrations/$${NEXT_NUM}_$(name).down.sql"

migrate-build: ## Build migration binary
	@echo "Building migration tool..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/migrate ./cmd/migrate

## Docker targets
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -t agentlink-api:latest -f Dockerfile.api .
	docker build -t agentlink-proxy:latest -f Dockerfile.proxy .

docker-up: ## Start Docker services
	@echo "Starting Docker services..."
	docker-compose up -d

docker-down: ## Stop Docker services
	@echo "Stopping Docker services..."
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

## Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## Development helpers
dev-api: ## Run API server with hot reload (requires air)
	@echo "Starting API server with hot reload..."
	air -c .air.api.toml

dev-proxy: ## Run Proxy Gateway with hot reload (requires air)
	@echo "Starting Proxy Gateway with hot reload..."
	air -c .air.proxy.toml

## Help
help: ## Display this help
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
