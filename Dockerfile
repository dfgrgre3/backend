# ==========================================
# Thanawy Backend - Docker Multi-Stage Build
# ==========================================
# Build stages:
#   1. builder      - Compile Go binaries
#   2. migrate      - Migration runner (small runtime)
#   3. seed         - Admin seeder (small runtime)
#   4. worker       - Background worker (includes asynq, redis)
#   5. api          - Main API server (smallest production image)
# ==========================================

# ------------------------------------------
# Stage 1: Builder - Compile all binaries
# ------------------------------------------
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API Builder"
LABEL version="1.0"

# Install build dependencies
RUN apk add --no-cache \
    git \
    build-base \
    ca-certificates

WORKDIR /build

# Copy dependency files first for layer caching
COPY go.mod go.sum ./

# Download dependencies (cached unless go.mod changes)
RUN go mod download

# Copy source code
COPY . .

# Generate swagger docs placeholder if not present
RUN mkdir -p docs && \
    printf 'package docs\n\n// Placeholder Swagger docs package generated at build time.\n' > docs/docs.go

# ------------------------------------------
# API Binary (Main Server)
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w \
        -X thanawy-backend/pkg/buildinfo.Version=${VERSION} \
        -X thanawy-backend/pkg/buildinfo.Commit=${COMMIT} \
        -X thanawy-backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /build/bin/api ./cmd/api/main.go

# ------------------------------------------
# Migrate Binary (Database Migrations)
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/migrate ./cmd/migrate/main.go

# ------------------------------------------
# Seed Admin Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/seed-admin ./cmd/seed-admin/main.go

# ------------------------------------------
# Targeted Migrate Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/targeted-migrate ./cmd/targeted_migrate/main.go

# ------------------------------------------
# Fix Migration Checksum Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/fix-migration-checksum ./cmd/fix-migration-checksum/main.go

# ------------------------------------------
# Cleanup-failed-migration Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/cleanup-failed-migration ./cmd/cleanup-failed-migration/main.go

# ------------------------------------------
# Drop-all-tables Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/drop-all-tables ./cmd/drop-all-tables/main.go

# ------------------------------------------
# Drop-migrations-table Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/drop-migrations-table ./cmd/drop-migrations-table/main.go

# ------------------------------------------
# Check-migration-status Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/check-migration-status ./cmd/check-migration-status/main.go

# ------------------------------------------
# Check-user Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/check-user ./cmd/check-user/main.go

# ------------------------------------------
# Test-db-connection Binary
# ------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o /build/bin/test-db-connection ./cmd/test-db-connection/main.go

# ------------------------------------------
# Stage 2: API Runtime - Main server
# ------------------------------------------
FROM alpine:3.20 AS api

ARG TARGETARCH

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API Runtime"
LABEL version="1.0"

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl

# Create non-root user
RUN adduser -D -h /app nonroot

# Copy certificates and binary
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/bin/api /app/main

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Set ownership
RUN chown -R nonroot:nonroot /app

USER nonroot

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD curl --silent --fail http://localhost:8082/health || exit 1

# Default command
CMD ["./main"]

# ------------------------------------------
# Stage 3: Migrate Runtime - Database migrations
# ------------------------------------------
FROM alpine:3.20 AS migrate

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend Migration Runner"
LABEL version="1.0"

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    tzdata

RUN adduser -D -h /app nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/bin/migrate /app/migrate
COPY --from=builder /build/bin/targeted-migrate /app/targeted-migrate
COPY --from=builder /build/bin/fix-migration-checksum /app/fix-migration-checksum
COPY --from=builder /build/bin/cleanup-failed-migration /app/cleanup-failed-migration
COPY --from=builder /build/bin/drop-all-tables /app/drop-all-tables
COPY --from=builder /build/bin/drop-migrations-table /app/drop-migrations-table
COPY --from=builder /build/bin/check-migration-status /app/check-migration-status
COPY --from=builder /build/bin/check-user /app/check-user
COPY --from=builder /build/bin/test-db-connection /app/test-db-connection
COPY --from=builder /build/bin/seed-admin /app/seed-admin

RUN chown -R nonroot:nonroot /app

USER nonroot

# Usage: docker run thanawy migrate [command]
# Commands: migrate, targeted-migrate, fix-migration-checksum, cleanup-failed-migration, drop-all-tables, drop-migrations-table
CMD ["echo"]


# ------------------------------------------
# Stage 4: Worker Runtime - Background workers
# ------------------------------------------
FROM alpine:3.20 AS worker

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend Worker"
LABEL version="1.0"

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    tzdata

RUN adduser -D -h /app nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/bin/api /app/worker

RUN chown -R nonroot:nonroot /app

USER nonroot

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD wget --spider -q http://localhost:8082/health || exit 1

# Run in worker mode: APP_MODE=worker ./worker
CMD ["./worker"]


# ------------------------------------------
# Stage 5: Developer Tools - Debug and dev
# ------------------------------------------
FROM builder AS developer

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend Developer Image"
LABEL version="1.0"

WORKDIR /app

COPY . .

# Install delve for debugging
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Expose delve debugger
EXPOSE 40000

# Default to running the API with dlv
CMD ["dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "run", "./cmd/api/main.go"]
