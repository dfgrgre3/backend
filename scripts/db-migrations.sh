#!/bin/bash
# Database migration management script
# Usage: ./scripts/db-migrations.sh [status|up|down|create|validate|rollback]

set -e

COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"
ACTION="${1:-status}"
MIGRATION_NAME="${2:-}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

# Check migration directory exists
ensure_migration_dir() {
    if [ ! -d "internal/infrastructure/database/migration/migrations" ]; then
        log_error "Migration directory not found"
        exit 1
    fi
}

# Show migration status
migration_status() {
    log_info "Checking migration status..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT 
    id,
    checksum,
    appliedAt,
    now() - appliedAt as age
FROM schema_migrations
ORDER BY appliedAt DESC
LIMIT 20;
EOF
}

# Apply all pending migrations
migration_up() {
    log_info "Applying pending migrations..."
    
    if ! $COMPOSE_CMD ps postgres | grep -q "Up"; then
        log_error "PostgreSQL is not running"
        exit 1
    fi
    
    if ! $COMPOSE_CMD exec -T postgres pg_isready -U thanawy > /dev/null 2>&1; then
        log_error "PostgreSQL is not ready"
        exit 1
    fi
    
    log_info "Running migrations..."
    if $COMPOSE_CMD run --rm migrate up; then
        log_success "Migrations applied successfully"
    else
        log_error "Migration failed"
        exit 1
    fi
}

# Validate migration files
validate_migrations() {
    log_info "Validating migration files..."
    ensure_migration_dir
    
    local count=0
    local errors=0
    
    while IFS= read -r -d '' file; do
        local filename=$(basename "$file")
        
        # Check filename format
        if ! [[ "$filename" =~ ^[0-9]{4}_.*\.sql$ ]]; then
            log_warn "Invalid filename format: $filename (should be: NNNN_description.sql)"
            ((errors++))
        fi
        
        # Check for non-UTF8 characters
        if ! file "$file" | grep -q "ASCII\|UTF-8"; then
            log_warn "File may contain non-UTF8 characters: $filename"
            ((errors++))
        fi
        
        # Check for common SQL syntax issues
        if grep -q "^[[:space:]]*$" "$file" | head -1 > /dev/null; then
            # File has content, check for syntax
            if ! grep -q ";" "$file"; then
                log_warn "Migration may be missing SQL statements: $filename"
                ((errors++))
            fi
        fi
        
        ((count++))
    done < <(find internal/infrastructure/database/migration/migrations -name "*.sql" -print0)
    
    log_success "Validated $count migration files with $errors warnings"
}

# Create a new migration
create_migration() {
    if [ -z "$MIGRATION_NAME" ]; then
        log_error "Migration name required"
        echo "Usage: $0 create <name>"
        exit 1
    fi
    
    ensure_migration_dir
    
    # Find next migration number
    local next_num=$(ls internal/infrastructure/database/migration/migrations/*.sql 2>/dev/null | \
        sed 's/.*\/\([0-9]*\)_.*/\1/' | sort -n | tail -1 || echo "0000")
    next_num=$((next_num + 1))
    next_num=$(printf "%04d" $next_num)
    
    local filename="internal/infrastructure/database/migration/migrations/${next_num}_${MIGRATION_NAME}.sql"
    
    if [ -f "$filename" ]; then
        log_error "Migration already exists: $filename"
        exit 1
    fi
    
    # Create template
    cat > "$filename" << 'EOF'
-- Migration: _MIGRATION_NAME_
-- Description: Brief description of the change
-- Created: _TIMESTAMP_

-- UP migration
BEGIN;

-- Add your SQL statements here
-- Example: ALTER TABLE users ADD COLUMN new_column TEXT DEFAULT 'value';

COMMIT;

-- DOWN migration (if rollback needed)
-- BEGIN;
-- -- Add rollback SQL here
-- COMMIT;
EOF
    
    sed -i "s/_MIGRATION_NAME_/$MIGRATION_NAME/g" "$filename"
    sed -i "s/_TIMESTAMP_/$(date)/g" "$filename"
    
    log_success "Created migration: $filename"
    cat "$filename"
}

# Check for migration issues
check_migration_health() {
    log_info "Checking migration system health..."
    
    # Check for duplicate migration names
    local duplicates=$(ls internal/infrastructure/database/migration/migrations/*.sql 2>/dev/null | \
        sed 's/.*\/\([0-9]*\)_.*/\1/' | sort | uniq -d)
    
    if [ -n "$duplicates" ]; then
        log_error "Duplicate migration numbers found: $duplicates"
        exit 1
    fi
    
    # Check for uncommitted migrations
    local pending=$($COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy -t -c \
        "SELECT COUNT(*) FROM schema_migrations WHERE appliedAt > now() - interval '1 minute';" 2>/dev/null || echo "0")
    
    if [ "$pending" -gt 0 ]; then
        log_info "Recent migrations: $pending"
    fi
    
    log_success "Migration health check passed"
}

# Rollback last migration (if supported)
rollback_migration() {
    log_warn "Rollback is manual - please use fix-migration-checksum or cleanup-failed-migration"
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT id FROM schema_migrations
ORDER BY appliedAt DESC
LIMIT 5;
EOF
}

# Main dispatch
case "$ACTION" in
    status)
        migration_status
        ;;
    up)
        migration_up
        ;;
    create)
        create_migration
        ;;
    validate)
        validate_migrations
        ;;
    health)
        check_migration_health
        ;;
    rollback)
        rollback_migration
        ;;
    *)
        log_error "Unknown action: $ACTION"
        echo "Available actions:"
        echo "  status       - Show migration status"
        echo "  up           - Apply pending migrations"
        echo "  create NAME  - Create new migration"
        echo "  validate     - Validate migration files"
        echo "  health       - Check migration system health"
        echo "  rollback     - Show last migrations"
        exit 1
        ;;
esac
