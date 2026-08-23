# Thanawy Backend

A high-performance Go backend API for the Thanawy education platform, built with Clean Architecture and Docker.

## Features

- **Go 1.26** - Latest stable version
- **Gin Framework** - High-performance HTTP router
- **GORM** - ORM with PostgreSQL
- **Redis** - Caching, rate limiting, workers, **and account-lockout brute-force protection** (see Security note below — this makes Redis a hard security dependency, not just a performance cache)
- **MinIO** - S3-compatible object storage
- **Clean Architecture** - Hexagonal architecture pattern
- **Docker Ready** - Full containerization support
- **JWT Authentication** - Secure token-based auth
- **Rate Limiting** - Built-in request throttling
- **CSRF Protection** - Security middleware
- **OpenTelemetry** - Distributed tracing

## Project Structure

```
D:\backend
├── cmd/                    # Application entry points
│   ├── api/               # Main API server
│   ├── migrate/           # Database migrations
│   ├── seed-admin/        # Admin user seeder
│   └── ...
├── internal/
│   ├── application/       # Business logic (CQRS)
│   ├── domain/            # Domain entities & services
│   └── infrastructure/    # External integrations
├── pkg/                   # Shared packages
│   ├── buildinfo/         # Version tracking
│   └── telemetry/         # OpenTelemetry setup
├── docs/                  # Swagger documentation
├── config/                # Configuration examples
├── scripts/               # Utility scripts
├── Dockerfile             # Multi-stage Docker build
├── docker-compose.yml     # Development environment
└── .env.example           # Environment template
```

## Quick Start with Docker

### Prerequisites

- Docker Engine 24.0+
- Docker Compose v2.20+

### Installation

```bash
# Copy environment templates
cp .env.docker.example .env.docker
cp .env.example .env

# Edit .env.docker with your values
# - Set POSTGRES_PASSWORD
# - Set MINIO_ROOT_PASSWORD

# Start all services
docker compose up -d

# Check status
docker compose ps
```

### Available Services

| Service | Port | Description |
|---------|------|-------------|
| postgres | 5432 | PostgreSQL 17 database |
| redis | 6379 | Redis 8 cache |
| minio | 9000 | S3-compatible storage |
| minio-console | 9001 | MinIO web UI |
| backend | 8082 | Go API server |
| mailpit (dev) | 8025 | Email testing UI |

### Common Commands

```bash
# Start with dev profile (includes Mailpit)
docker compose up -d --profile dev

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v

# View logs
docker compose logs -f

# Run migrations
docker compose run --rm migrate up

# Open shell in container
docker compose exec backend sh
```

## Local Development (Without Docker)

### Prerequisites

- Go 1.26+
- PostgreSQL 17+
- Redis 8+
- MinIO (for S3 storage)

### Setup

```bash
# Copy environment template
cp .env.example .env

# Edit .env with your local configuration

# Install dependencies
go mod download

# Run migrations
go run ./cmd/migrate/main.go up

# Seed admin user (optional)
go run ./cmd/seed-admin/main.go

# Start server
go run ./cmd/api/main.go
```

The API will be available at `http://localhost:8082`

## Environment Variables

### Infrastructure (.env.docker)

| Variable | Required | Default |
|----------|----------|---------|
| POSTGRES_USER | No | thanawy |
| POSTGRES_PASSWORD | Yes | - |
| POSTGRES_DB | No | thanawy |
| MINIO_ROOT_PASSWORD | Yes | - |

### Application (.env)

Key variables to configure:

```env
# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/thanawy

# JWT
JWT_SECRET=your_32_byte_secret_key
JWT_ISSUER_URL=http://localhost:8082

# Storage (S3)
STORAGE_TYPE=s3
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=your_password
S3_BUCKET=thanawy

# Redis
REDIS_URL=redis://localhost:6379

# Email
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=your_user
SMTP_PASS=your_password
```

See `.env.example` for complete configuration options.

> **Security note — `REDIS_URL` is required in production, not optional.**
> Login brute-force protection (account lockout after repeated failed
> attempts) is implemented entirely via Redis
> (`internal/domain/auth/service/auth_service.go`). If Redis is unreachable
> or unconfigured, that check fails **open** — no lockout is enforced and no
> error is raised — rather than blocking logins. Never run production
> without Redis configured.

## API Documentation

Once the server is running, visit:

- **Swagger UI**: `http://localhost:8082/swagger`
- **Health Check**: `http://localhost:8082/health`
- **Ready Check**: `http://localhost:8082/api/readyz`

## Docker Profiles

### Development Profile

```bash
# Includes Mailpit for email testing
docker compose up -d --profile dev
```

Mailpit UI: `http://localhost:8025`

### Production Profile

1. Create `.env.production` with secure values
2. Update CORS_ORIGINS for your domain
3. Set S3_USE_SSL=true
4. Configure production S3 credentials
5. Run:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Building Images

```bash
# Build all images
docker compose build

# Build specific service
docker compose build backend

# Build with version info
docker compose build \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT=$(git rev-parse HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  backend
```

## Migrations

```bash
# Run all pending migrations
docker compose run --rm migrate up

# Create new migration
# (Use your migration tool of choice)

# Check migration status
docker compose run --rm migrate status

# Force migration version
docker compose run --rm migrate force <version>
```

## Troubleshooting

### Container won't start

```bash
# Check logs
docker compose logs backend

# Check health
docker compose ps
```

### Database connection issues

```bash
# Verify postgres is healthy
docker compose ps postgres

# Test connection
docker compose exec postgres psql -U thanawy -d thanawy
```

### Clear all data

```bash
docker compose down -v
docker volume prune
docker compose up -d
```

## Deployment

See [DOCKER_README.md](DOCKER_README.md) for comprehensive deployment guide.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Run lint: `golangci-lint run`
6. Submit a pull request

## License

MIT License - See LICENSE file for details.

## Support

For issues and questions:
1. Check the logs first
2. Review this documentation
3. Check individual service docs (PostgreSQL, Redis, MinIO)
