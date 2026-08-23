# ==========================================
# Thanawy Backend - Secure Multi-Stage Build
# ==========================================

# ------------------------------------------
# Stage 1: Builder - Secure Compilation
# ------------------------------------------
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API Builder (Secure)"
LABEL version="1.0"

# Install build dependencies
RUN apk add --no-cache git build-base ca-certificates

WORKDIR /build

# Copy dependency files first for optimal layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate swagger docs placeholder if not present
RUN mkdir -p docs && \
    printf 'package docs\n\n// Placeholder Swagger docs package generated at build time.\n' > docs/docs.go

# Compile all binaries with security flags:
# -trimpath: Removes local file system paths from the compiled binary (prevents info leakage)
# -s -w: Strips symbol table and debug info (reduces size and reverse engineering risk)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w \
        -X thanawy-backend/pkg/buildinfo.Version=${VERSION} \
        -X thanawy-backend/pkg/buildinfo.Commit=${COMMIT} \
        -X thanawy-backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /build/bin/api ./cmd/api/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/migrate ./cmd/migrate/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/seed-admin ./cmd/seed-admin/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/targeted-migrate ./cmd/targeted_migrate/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/fix-migration-checksum ./cmd/fix-migration-checksum/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/cleanup-failed-migration ./cmd/cleanup-failed-migration/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/drop-all-tables ./cmd/drop-all-tables/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/drop-migrations-table ./cmd/drop-migrations-table/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/check-migration-status ./cmd/check-migration-status/main.go && \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" \
    -o /build/bin/test-db-connection ./cmd/test-db-connection/main.go

# ------------------------------------------
# Stage 2: API Runtime - Main server (Hardened)
# ------------------------------------------
FROM alpine:3.20 AS api

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API Runtime (Hardened)"
LABEL version="1.0"

WORKDIR /app

# Install minimal runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user with NO shell access (-s /sbin/nologin) and no home directory (-H)
RUN adduser -D -H -h /app -s /sbin/nologin nonroot

# Copy only the necessary binary
COPY --from=builder /build/bin/api /app/main

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Set strict ownership and read-only permissions for the binary
RUN chown -R nonroot:nonroot /app && \
    chmod 555 /app/main

USER nonroot

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD curl --silent --fail http://localhost:8082/health || exit 1

# Use entrypoint to validate environment before starting
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["./main"]

# ------------------------------------------
# Stage 3: Migrate Runtime - Database tools (Hardened)
# ------------------------------------------
FROM alpine:3.20 AS migrate

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend Migration Runner (Hardened)"
LABEL version="1.0"

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -H -h /app -s /sbin/nologin nonroot

# Copy all database management binaries
COPY --from=builder /build/bin/migrate /app/migrate
COPY --from=builder /build/bin/targeted-migrate /app/targeted-migrate
COPY --from=builder /build/bin/fix-migration-checksum /app/fix-migration-checksum
COPY --from=builder /build/bin/cleanup-failed-migration /app/cleanup-failed-migration
COPY --from=builder /build/bin/drop-all-tables /app/drop-all-tables
COPY --from=builder /build/bin/drop-migrations-table /app/drop-migrations-table
COPY --from=builder /build/bin/check-migration-status /app/check-migration-status
COPY --from=builder /build/bin/test-db-connection /app/test-db-connection
COPY --from=builder /build/bin/seed-admin /app/seed-admin

# SQL migration files (read at runtime, relative to WORKDIR)
COPY --from=builder /build/internal/infrastructure/database/migration/migrations /app/internal/infrastructure/database/migration/migrations

# Set strict ownership and read-only permissions
RUN chown -R nonroot:nonroot /app && \
    chmod 555 /app/* && \
    chmod -R 555 /app/internal

USER nonroot

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/app/migrate", "--help"]

# ------------------------------------------
# Stage 4: Worker Runtime - Background workers (Hardened)
# ------------------------------------------
FROM alpine:3.20 AS worker

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend Worker (Hardened)"
LABEL version="1.0"

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -H -h /app -s /sbin/nologin nonroot

# Copy the dedicated worker binary
COPY --from=builder /build/bin/worker /app/worker

RUN chown -R nonroot:nonroot /app && \
    chmod 555 /app/worker

USER nonroot

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD wget --spider -q http://localhost:8082/health || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
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

EXPOSE 40000

CMD ["dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "run", "./cmd/api/main.go"]