# Thanawy Backend Architecture

## Overview

The Thanawy backend is a Go-based REST API serving an educational platform. It follows a **layered architecture with CQRS patterns** and uses **GORM** for database access with **PostgreSQL** as the primary database.

## Architecture Patterns

### 1. Layered Architecture

The project follows a traditional layered architecture with clear separation of concerns:

```
cmd/api/              → Application entry point
internal/
├── api/handlers/     → HTTP request handlers (presentation layer)
├── services/         → Business logic layer
├── repository/       → Data access layer
├── models/           → Domain models/entities
├── db/               → Database connection and migrations
├── middleware/       → HTTP middleware (auth, CORS, rate limiting)
├── router/           → Route definitions
└── cqrs/             → CQRS command/query handlers
```

### 2. CQRS (Command Query Responsibility Segregation)

The project implements CQRS patterns for read/write separation:

- **Read Path**: Uses `db.ReadDB()` to route queries to read replicas
- **Write Path**: Uses `db.WriteDB()` to route commands to the primary database
- **Commands**: Located in `internal/cqrs/commands/`
- **Queries**: Located in `internal/cqrs/queries/`

### 3. Repository Pattern

Data access is abstracted through repository interfaces in `internal/repository/`:

- `user_repo.go` - User data access
- `subject_repo.go` - Subject data access
- `auth_repo.go` - Authentication data access
- `lms_repo.go` - LMS-specific data access
- And 5 additional repositories for various domains

### 4. Dependency Injection

Composition is performed explicitly in `internal/app/wire.go`. New handlers,
services, and repositories must receive database, cache, and publisher
dependencies through constructors. The legacy `db.DB` and `db.Redis` package
globals remain only as a compatibility bridge while existing modules are moved
incrementally; new code must not depend on them.

## Database Architecture

### Primary Database System

**GORM with PostgreSQL** is the primary database system:

- **Migrations**: SQL migrations in `internal/db/migrations/` (41 migration files)
- **ORM**: GORM for runtime database operations
- **Connection Pooling**: Configured for both serverless and traditional environments
- **Read Replicas**: Support for read replica routing via `DATABASE_REPLICAS` env var

### Historical Context

The project previously had multiple database systems which have been consolidated:

- ~~Prisma~~ - Removed (was used only as CLI tool, now unnecessary)
- ~~Supabase~~ - Removed (direct PostgreSQL connection used instead)
- **GORM** - Kept as the primary ORM

### Migration Strategy

- SQL-based migrations in `internal/db/migrations/`
- Numbered migration files (0000_baseline_schema.sql, 0001_add_user_session.sql, etc.)
- Migration tool: `cmd/migrate/main.go`
- **Schema authority**: these numbered SQL files are the only production schema authority.
- GORM `AutoMigrate` is not permitted in API or worker startup paths.
- Prisma and Supabase migration directories are historical and must not be reintroduced.
- The current custom runner is transitional. Its advisory lock and checksum guarantees
  must be preserved when replacing it with `goose` or `golang-migrate`.

## Key Components

### Handlers (75+ files)

HTTP handlers in `internal/api/handlers/` handle API endpoints:

- `auth_handler.go` - Authentication endpoints
- `user_handler.go` - User management
- `subject_handler.go` - Subject management
- `course_rest_handler.go` - Course operations
- `admin_*_handler.go` - Admin-specific endpoints
- And 70+ additional handlers

### Services (28 files)

Business logic in `internal/services/`:

- `auth_service.go` - Authentication business logic
- `email_service.go` - Email operations
- `ai_service.go` - AI integration
- `payment_service.go` - Payment processing
- And 24 additional services

### Models (38 files)

Domain models in `internal/models/`:

- `user.go` - User entity
- `subject.go` - Subject entity
- `auth_models.go` - Authentication models
- `lms.go` - LMS-specific models
- And 34 additional model files

### Middleware (23 files)

HTTP middleware in `internal/middleware/`:

- Authentication and authorization
- CORS handling
- Rate limiting
- CSRF protection
- Security validation

## Storage

### S3-Compatible Storage

File storage uses S3-compatible services (Cloudflare R2, AWS S3, or MinIO):

- Configured via `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`
- Public URL configuration via `S3_PUBLIC_URL`
- SSL toggle via `S3_USE_SSL`

## Caching

### Redis

Redis is used for caching and session management:

- Configured via `REDIS_URL` environment variable
- Connection in `internal/db/redis.go`
- Request/task contexts are passed by callers; package-level background contexts are prohibited.
- The client is closed during graceful shutdown and connection pool/retry values are configurable.

## Background Processing

### Workers

Background job processing in `internal/worker/`:

- `scheduler.go` - Job scheduling
- `exam.go` - Exam-related background tasks
- And 10 additional worker files

## API Documentation

Swagger/OpenAPI documentation is available in `docs/`:

- `swagger.json` - OpenAPI specification
- `swagger.yaml` - YAML version of the spec

## Configuration

Configuration is managed through environment variables (see `.env.example`):

- Database settings
- JWT configuration
- Storage settings
- HTTP server timeouts
- Rate limiting
- Cookie security

## Development Tools

### Makefile

Common operations via Makefile:

- `make build` - Build the application
- `make test` - Run tests
- `make lint` - Run linters
- `make migrate-up` - Run database migrations
- `make migrate-down` - Rollback migrations
- `make run` - Run the API server
- `make dev` - Run with hot reload (requires air)

### Tools Directory

Development and diagnostic tools in `tools/`:

- `deployment/` - Deployment scripts
- `dev/` - Development utilities
- `diagnostics/` - Diagnostic tools
- `check_db_constraints/` - Database constraint checker
- `test_cascade/` - Cascade testing tool

## CI/CD

GitHub Actions workflow in `.github/workflows/ci.yml`:

- Runs tests on push/PR
- Uses PostgreSQL and Redis services
- Runs go vet, go fmt, golangci-lint
- Uploads coverage to Codecov

## Code Quality

### Linting

golangci-lint configuration in `.golangci.yml`:

- 19 enabled linters
- Custom rules for test files
- Excludes migration files from linting

### Git Ignore

Comprehensive `.gitignore` covering:

- Go build artifacts
- Node.js artifacts
- Python artifacts
- IDE configurations
- Temporary files
- Generated code

## Security Features

- BCrypt password hashing (configurable cost)
- JWT token validation with JWKS
- CSRF protection
- Rate limiting
- IP whitelisting
- Secure cookie configuration
- SQL injection prevention (via GORM)
- Security logging

## Performance Optimizations

- Database connection pooling
- Read replica routing
- Prepared statement caching (when not using PgBouncer)
- Serverless-optimized connection pools
- Redis caching
- Materialized views (in migrations)

## Deployment

The supported production target is a long-running container or VM (Docker,
Kubernetes, or equivalent). This is required for native gRPC, WebSockets,
background workers, schedulers, and predictable graceful shutdown.

Vercel support is a compatibility-only REST surface. It must not run workers,
schedulers, native gRPC, or rely on long-lived WebSocket connections. New
production deployments must not use Vercel for the Go process.

## API Transport Direction

- REST is the primary public transport.
- Native gRPC and Connect-RPC are compatibility transports and must be enabled
  explicitly per deployment.
- New endpoints should be implemented in REST first. RPC expansion requires an
  approved client/use case and ownership plan.
- The `/api` prefix rewrite is a Vercel compatibility adapter, not a routing
  convention for long-running deployments.

## Infrastructure Refactoring Roadmap

1. Move repositories and services domain-by-domain from `db.DB`/`db.Redis` to
   constructor-injected dependencies; remove globals only after the final caller.
2. Replace direct Redis access with small capability interfaces (cache,
   idempotency, stream consumer) that accept caller contexts.
3. Replace the custom SQL splitter with a maintained migration tool after a
   production migration-table compatibility test; retain checksums and locking.
4. Replace no-op event publishers with an outbox-backed publisher or remove the
   event contract. Silent event loss is not an acceptable production default.
5. Remove Vercel-only routing after traffic has moved to the supported container target.
