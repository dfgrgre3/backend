.PHONY: help build test lint fmt clean migrate-up run dev install-deps docker-up docker-down docker-down-v docker-ps docker-logs docker-restart backend frontend logs redis-cli postgres clean

# Variables
APP_NAME=thanawy-backend
CMD_DIR=./cmd
API_CMD=$(CMD_DIR)/api
MIGRATE_CMD=$(CMD_DIR)/migrate
BUILD_DIR=./bin
GO=go

# Default target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Building & Development:"
	@echo "  make build         - Build the application"
	@echo "  make test          - Run tests"
	@echo "  make lint          - Run linters"
	@echo "  make fmt           - Format code"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make run           - Run the API server"
	@echo "  make dev           - Run in development mode with hot reload"
	@echo "  make install-deps  - Install Go dependencies"
	@echo ""
	@echo "Docker Infrastructure:"
	@echo "  make docker-up     - Start infrastructure (Postgres, Redis, MinIO, Mailpit)"
	@echo "  make docker-down   - Stop infrastructure (keeps data)"
	@echo "  make docker-down-v - Stop infrastructure and delete data (WARNING)"
	@echo "  make docker-ps     - Show infrastructure status"
	@echo "  make docker-logs   - Follow infrastructure logs"
	@echo "  make docker-restart - Restart infrastructure"
	@echo ""
	@echo "Database Management:"
	@echo "  make db-health     - Check database health"
	@echo "  make db-migrate    - Run database migrations"
	@echo "  make db-status     - Show migration status"
	@echo "  make db-optimize   - Run database optimization"
	@echo "  make db-backup     - Backup database to SQL file"
	@echo "  make db-shell      - Open PostgreSQL shell"
	@echo "  make redis-cli     - Open Redis CLI"
	@echo ""
	@echo "Services:"
	@echo "  make backend       - Start backend service"
	@echo "  make frontend      - Start frontend service"
	@echo "  make logs          - Follow all service logs"

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/api $(API_CMD)/main.go
	$(GO) build -o $(BUILD_DIR)/migrate $(MIGRATE_CMD)
	@echo "Build complete: $(BUILD_DIR)/api, $(BUILD_DIR)/migrate"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Test coverage report: coverage.out"

# Run linters
lint:

# ============================================
# Database Management Targets
# ============================================

# Check database health
db-health:
	@bash ./scripts/db-health-check.sh all

# Run database migrations
db-migrate:
	docker compose run --rm migrate up

# Show migration status
db-status:
	docker compose exec postgres psql -U thanawy -d thanawy -c "SELECT id, checksum, appliedAt FROM schema_migrations ORDER BY appliedAt DESC LIMIT 20;"

# Optimize database
db-optimize:
	@bash ./scripts/db-optimize.sh all

# Backup database to SQL
db-backup:
	@mkdir -p ./backups
	@docker compose exec -T postgres pg_dump -U thanawy thanawy > ./backups/backup_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "Database backed up to ./backups/"

# Open PostgreSQL shell
db-shell:
	docker compose exec postgres psql -U thanawy -d thanawy

# View database logs
db-logs:
	docker compose logs -f postgres
	@echo "Running linters..."
	$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@echo "Clean complete"

# Run database migrations up
migrate-up:
	@echo "Running database migrations..."
	$(GO) run $(MIGRATE_CMD)

# Run the API server
run:
	@echo "Starting API server..."
	$(GO) run $(API_CMD)/main.go

# Run in development mode with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install it with: go install github.com/cosmtrek/air@latest"; \
		$(GO) run $(API_CMD)/main.go; \
	fi

# Install Go dependencies
install-deps:
	@echo "Installing Go dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies installed"

# ==========================================
# Docker Compose (Development Infrastructure)
# ==========================================
# Start all infrastructure services (Postgres, Redis, MinIO, Mailpit)
docker-up:
	@echo "Starting infrastructure services (Postgres, Redis, MinIO, Mailpit)..."
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker up -d; \
	else \
		docker compose up -d; \
	fi
	@echo "Services started. Check status with: make docker-ps"

# Stop all infrastructure services (keeps data volumes)
docker-down:
	@echo "Stopping infrastructure services..."
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker down; \
	else \
		docker compose down; \
	fi
	@echo "Services stopped. Data volumes preserved."

# Stop services AND remove data volumes (WARNING: deletes all data)
docker-down-v:
	@echo "WARNING: This will delete all data volumes!"
	@read -p "Type 'yes' to confirm: " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		if [ -f .env.docker ]; then \
			docker compose --env-file .env.docker down -v; \
		else \
			docker compose down -v; \
		fi; \
		echo "Services stopped and volumes removed."; \
	else \
		echo "Aborted."; \
	fi

# Show status of infrastructure services
docker-ps:
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker ps; \
	else \
		docker compose ps; \
	fi

# Follow logs of infrastructure services
docker-logs:
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker logs -f; \
	else \
		docker compose logs -f; \
	fi

# Restart all infrastructure services
docker-restart:
	@echo "Restarting infrastructure services..."
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker restart; \
	else \
		docker compose restart; \
	fi
	@echo "Services restarted."

# Start backend service
backend:
	@echo "Starting backend service..."
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker up -d backend; \
	else \
		docker compose up -d backend; \
	fi

# Start frontend service
frontend:
	@echo "Starting frontend service..."
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker up -d frontend; \
	else \
		docker compose up -d frontend; \
	fi

# Follow all service logs
logs:
	@if [ -f .env.docker ]; then \
		docker compose --env-file .env.docker logs -f --tail=100; \
	else \
		docker compose logs -f --tail=100; \
	fi

# Open Redis CLI
redis-cli:
	@docker exec -it thanawy-redis redis-cli

# Open PostgreSQL shell
postgres:
	@if [ -f .env.docker ]; then \
		docker exec -it thanawy-postgres psql -U $$(grep POSTGRES_USER .env.docker | cut -d= -f2) -d $$(grep POSTGRES_DB .env.docker | cut -d= -f2); \
	else \
		docker exec -it thanawy-postgres psql -U thanawy -d thanawy; \
	fi

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/cosmtrek/air@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed"

# Install gitleaks + register the pre-commit secret scan hook.
# Idempotent: safe to run multiple times.
install-hooks:
	@echo "Installing gitleaks pre-commit hook..."
	@if ! command -v gitleaks >/dev/null 2>&1; then \
		echo ""; \
		echo "  WARNING: gitleaks is not on PATH."; \
		echo "  Install with one of:"; \
		echo "    brew install gitleaks                 # macOS"; \
		echo "    go install github.com/gitleaks/gitleaks/v8@latest"; \
		echo "    scoop install gitleaks                # Windows"; \
		echo ""; \
	fi
	@mkdir -p .githooks
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "Pre-commit hook registered at .githooks/pre-commit"
	@echo "Every 'git commit' will now scan staged changes for secrets."

# Run gitleaks against the full git history. Useful for a one-off audit
# of an existing repo, or to verify the config behaves as expected.
gitleaks-scan:
	@if ! command -v gitleaks >/dev/null 2>&1; then \
		echo "gitleaks not installed. See 'make install-hooks' for install instructions."; \
		exit 1; \
	fi
	gitleaks detect --source . --redact --verbose --config .gitleaks.toml
