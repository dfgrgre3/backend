.PHONY: help build test lint fmt clean migrate-up migrate-down run dev install-deps docker-up docker-down docker-down-v docker-ps docker-logs docker-restart backend frontend logs redis-cli postgres clean

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
	@echo "  make build         - Build the application"
	@echo "  make test          - Run tests"
	@echo "  make lint          - Run linters"
	@echo "  make fmt           - Format code"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make migrate-up    - Run database migrations"
	@echo "  make migrate-down  - Rollback database migrations"
	@echo "  make run           - Run the API server"
	@echo "  make dev           - Run in development mode with hot reload"
	@echo "  make install-deps  - Install Go dependencies"
	@echo "  make docker-up     - Start infrastructure (Postgres, Redis, MinIO, Mailpit)"
	@echo "  make docker-down   - Stop infrastructure (keeps data)"
	@echo "  make docker-down-v - Stop infrastructure and delete data (WARNING)"
	@echo "  make docker-ps     - Show infrastructure status"
	@echo "  make docker-logs   - Follow infrastructure logs"
	@echo "  make docker-restart - Restart infrastructure"
	@echo "  make backend       - Start backend service"
	@echo "  make frontend      - Start frontend service"
	@echo "  make logs          - Follow all service logs"
	@echo "  make redis-cli     - Open Redis CLI"
	@echo "  make postgres      - Open PostgreSQL shell"

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/api $(API_CMD)/main.go
	$(GO) build -o $(BUILD_DIR)/migrate $(MIGRATE_CMD)/main.go
	@echo "Build complete: $(BUILD_DIR)/api, $(BUILD_DIR)/migrate"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Test coverage report: coverage.out"

# Run linters
lint:
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
	$(GO) run $(MIGRATE_CMD)/main.go up

# Run database migrations down
migrate-down:
	@echo "Rolling back database migrations..."
	$(GO) run $(MIGRATE_CMD)/main.go down

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
