# =============================================================================
# Distributed Job Scheduler — Makefile
# =============================================================================

.PHONY: help dev up down build migrate lint test clean

# ── Config ────────────────────────────────────────────────────────────────────
COMPOSE      = docker compose
COMPOSE_DEV  = $(COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml
BACKEND_DIR  = ./backend
MIGRATE_DIR  = $(BACKEND_DIR)/migrations
GO           = cd $(BACKEND_DIR) && go

# ── Help ──────────────────────────────────────────────────────────────────────
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Development ───────────────────────────────────────────────────────────────
dev: ## Start all services in dev mode (hot reload)
	$(COMPOSE_DEV) up --build

up: ## Start all services in production mode
	$(COMPOSE) up --build -d

down: ## Stop all services and remove containers
	$(COMPOSE) down

restart: ## Restart a specific service: make restart svc=api
	$(COMPOSE) restart $(svc)

scale-workers: ## Scale workers: make scale-workers n=3
	$(COMPOSE) up --scale worker=$(n) -d --no-recreate

# ── Building ──────────────────────────────────────────────────────────────────
build: ## Build backend binaries locally
	$(GO) build ./cmd/api
	$(GO) build ./cmd/worker

build-docker: ## Build all Docker images
	$(COMPOSE) build

# ── Database ──────────────────────────────────────────────────────────────────
migrate-up: ## Run all pending migrations
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path $(MIGRATE_DIR) \
		-database "$$(grep POSTGRES_URL .env | cut -d= -f2)" \
		up

migrate-down: ## Roll back the last migration
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path $(MIGRATE_DIR) \
		-database "$$(grep POSTGRES_URL .env | cut -d= -f2)" \
		down 1

migrate-status: ## Show migration status
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path $(MIGRATE_DIR) \
		-database "$$(grep POSTGRES_URL .env | cut -d= -f2)" \
		version

# ── Code Quality ──────────────────────────────────────────────────────────────
lint: ## Run golangci-lint
	cd $(BACKEND_DIR) && golangci-lint run ./...

fmt: ## Format all Go code
	cd $(BACKEND_DIR) && gofmt -w .

vet: ## Run go vet
	$(GO) vet ./...

# ── Testing ───────────────────────────────────────────────────────────────────
test: ## Run all tests
	$(GO) test -v -race -count=1 ./...

test-short: ## Run only short tests (no DB/Redis required)
	$(GO) test -v -short -count=1 ./...

test-coverage: ## Run tests with coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: backend/coverage.html"

# ── Dependencies ──────────────────────────────────────────────────────────────
deps: ## Download and tidy Go dependencies
	$(GO) mod download
	$(GO) mod tidy

# ── Cleanup ───────────────────────────────────────────────────────────────────
clean: ## Remove build artifacts and stop containers
	$(COMPOSE) down -v --remove-orphans
	rm -f $(BACKEND_DIR)/api $(BACKEND_DIR)/worker
	rm -f $(BACKEND_DIR)/coverage.out $(BACKEND_DIR)/coverage.html

# ── Setup ─────────────────────────────────────────────────────────────────────
setup: ## First-time setup: copy .env.example to .env
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example — please update secrets!"; \
	else \
		echo ".env already exists, skipping."; \
	fi
