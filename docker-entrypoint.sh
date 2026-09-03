#!/bin/sh
# ==========================================
# Thanawy Backend - Docker Entrypoint
# ==========================================
# This script runs before the main container process starts.
# It handles database migrations and initial setup.
# ==========================================

set -e

# ==========================================
# Validate Critical Environment Variables
# ==========================================
if [ -z "${DATABASE_HOST}" ]; then
    echo "[ENTRYPOINT] FATAL: DATABASE_HOST is not set"
    exit 1
fi

if [ -z "${POSTGRES_USER}" ]; then
    echo "[ENTRYPOINT] FATAL: POSTGRES_USER is not set"
    exit 1
fi

if [ -z "${POSTGRES_DB}" ]; then
    echo "[ENTRYPOINT] FATAL: POSTGRES_DB is not set"
    exit 1
fi

if [ -z "${POSTGRES_PASSWORD}" ]; then
    echo "[ENTRYPOINT] FATAL: POSTGRES_PASSWORD is not set"
    exit 1
fi

# Set default port if not provided
if [ -z "${DATABASE_PORT}" ]; then
    DATABASE_PORT=5432
fi

echo "=========================================="
echo "Thanawy Backend - Docker Entrypoint"
echo "Timestamp: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "=========================================="

# Function to check if database is ready
wait_for_database() {
    local max_attempts=30
    local attempt=1

    echo "[ENTRYPOINT] Waiting for database to be ready..."
    echo "[ENTRYPOINT] DATABASE_HOST=${DATABASE_HOST}"
    echo "[ENTRYPOINT] DATABASE_PORT=${DATABASE_PORT}"
    echo "[ENTRYPOINT] POSTGRES_USER=${POSTGRES_USER}"
    echo "[ENTRYPOINT] POSTGRES_DB=${POSTGRES_DB}"

    while [ $attempt -le $max_attempts ]; do
        if command -v psql > /dev/null 2>&1; then
            if PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${DATABASE_HOST}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c "SELECT 1" > /dev/null 2>&1; then
                echo "[ENTRYPOINT] Database is ready!"
                return 0
            fi
        elif [ -x /app/migrate ]; then
            if /app/migrate status > /dev/null 2>&1; then
                echo "[ENTRYPOINT] Database is ready!"
                return 0
            fi
        elif command -v nc > /dev/null 2>&1; then
            if nc -z "${DATABASE_HOST}" "${DATABASE_PORT}" > /dev/null 2>&1; then
                echo "[ENTRYPOINT] Database port is open"
                return 0
            fi
        fi

        echo "[ENTRYPOINT] Database not ready (attempt $attempt/$max_attempts)..."
        sleep 2
        attempt=$((attempt + 1))
    done

    echo "[ENTRYPOINT] ERROR: Database did not become ready in time"
    return 1
}

# Function to run migrations
run_migrations() {
    echo "[ENTRYPOINT] Running database migrations..."

    if [ -f "/app/migrate" ]; then
        # Check if migrations should be skipped
        if [ "${SKIP_AUTO_MIGRATE:-false}" = "true" ]; then
            echo "[ENTRYPOINT] SKIP_AUTO_MIGRATE=true, skipping migrations"
            return 0
        fi

        # Check if running in migration-only mode
        if [ "${MODE:-api}" = "migrate" ] || [ "${APP_MODE:-api}" = "migrate" ]; then
            echo "[ENTRYPOINT] Running migrations and exiting..."
            /app/migrate up
            exit 0
        fi

        # Run migrations before starting API
        /app/migrate up
        echo "[ENTRYPOINT] Migrations completed successfully"
    else
        echo "[ENTRYPOINT] WARNING: migrate binary not found, skipping migrations"
    fi
}

# Function to seed admin user
seed_admin_if_needed() {
    echo "[ENTRYPOINT] Checking if admin user needs to be seeded..."

    # Only seed if DEFAULT_ADMIN_PASSWORD is set and not empty
    if [ -n "${DEFAULT_ADMIN_PASSWORD:-}" ]; then
        if [ -f "/app/seed-admin" ]; then
            /app/seed-admin
            echo "[ENTRYPOINT] Admin user check complete"
        else
            echo "[ENTRYPOINT] WARNING: seed-admin binary not found"
        fi
    else
        echo "[ENTRYPOINT] DEFAULT_ADMIN_PASSWORD not set, skipping admin seeding"
    fi
}

# Function to check migration status
check_migration_status() {
    echo "[ENTRYPOINT] Checking migration status..."

    if [ -f "/app/check-migration-status" ]; then
        /app/check-migration-status || true
    else
        echo "[ENTRYPOINT] WARNING: check-migration-status binary not found"
    fi
}

# Function to cleanup failed migrations
cleanup_failed_migrations() {
    echo "[ENTRYPOINT] Checking for failed migrations..."

    if [ -f "/app/cleanup-failed-migration" ]; then
        /app/cleanup-failed-migration || true
    else
        echo "[ENTRYPOINT] WARNING: cleanup-failed-migration binary not found"
    fi
}

# Main entrypoint logic
main() {
    echo "[ENTRYPOINT] Starting entrypoint script..."
    echo "[ENTRYPOINT] Container command: $@"

    # Wait for database
    wait_for_database

    # Check and cleanup failed migrations if needed
    cleanup_failed_migrations

    # Run migrations (unless skipped)
    run_migrations

    # Seed admin user if password is provided
    seed_admin_if_needed

    # Show migration status
    check_migration_status

    echo "[ENTRYPOINT] Entrypoint complete. Starting main process..."
    echo "=========================================="

    # Execute the main command passed to docker run
    exec "$@"
}

# Run main function with all arguments
main "$@"
