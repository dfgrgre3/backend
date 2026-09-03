#!/bin/bash
# Database health check and diagnostics script
# Usage: ./scripts/db-health-check.sh [postgres|redis|minio|all]

set -e

COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"
TARGET="${1:-all}"
TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# PostgreSQL Health Check
check_postgres() {
    log_info "Checking PostgreSQL health..."
    
    if ! $COMPOSE_CMD ps postgres | grep -q "Up"; then
        log_error "PostgreSQL container is not running"
        return 1
    fi
    log_success "PostgreSQL container is running"
    
    # Test connection
    if $COMPOSE_CMD exec -T postgres pg_isready -U thanawy > /dev/null 2>&1; then
        log_success "PostgreSQL connection successful"
    else
        log_error "PostgreSQL connection failed"
        return 1
    fi
    
    # Check connections
    log_info "Active connections:"
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy -c \
        "SELECT datname, count(*) as connections FROM pg_stat_activity GROUP BY datname;" 2>/dev/null || true
    
    # Check disk usage
    log_info "Database size:"
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy -c \
        "SELECT pg_size_pretty(pg_database_size(current_database())) as size;" 2>/dev/null || true
    
    # Check for idle transactions
    log_info "Checking for long-running queries..."
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy -c \
        "SELECT pid, usename, now() - query_start as duration, query 
         FROM pg_stat_activity 
         WHERE (now() - query_start) > interval '5 minutes' 
         LIMIT 5;" 2>/dev/null || true
    
    # Migration status
    log_info "Applied migrations:"
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy -c \
        "SELECT COUNT(*) as total_migrations FROM schema_migrations;" 2>/dev/null || true
    
    return 0
}

# Redis Health Check
check_redis() {
    log_info "Checking Redis health..."
    
    if ! $COMPOSE_CMD ps redis | grep -q "Up"; then
        log_error "Redis container is not running"
        return 1
    fi
    log_success "Redis container is running"
    
    # Test connection
    if $COMPOSE_CMD exec -T redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" ping > /dev/null 2>&1; then
        log_success "Redis connection successful"
    else
        log_error "Redis connection failed"
        return 1
    fi
    
    # Check memory usage
    log_info "Redis memory usage:"
    $COMPOSE_CMD exec -T redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" INFO memory 2>/dev/null | \
        grep -E "used_memory|used_memory_human" || true
    
    # Check key count
    log_info "Redis key statistics:"
    $COMPOSE_CMD exec -T redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" DBSIZE 2>/dev/null || true
    
    return 0
}

# MinIO Health Check
check_minio() {
    log_info "Checking MinIO health..."
    
    if ! $COMPOSE_CMD ps minio | grep -q "Up"; then
        log_warn "MinIO container is not running (optional)"
        return 0
    fi
    log_success "MinIO container is running"
    
    # Check if bucket exists
    log_info "MinIO buckets:"
    $COMPOSE_CMD exec -T minio mc ls local 2>/dev/null || log_warn "Could not list buckets"
    
    return 0
}

# Full diagnostics
run_full_diagnostics() {
    log_info "========================================"
    log_info "Database Health Check - $TIMESTAMP"
    log_info "========================================"
    echo ""
    
    case "$TARGET" in
        postgres)
            check_postgres
            ;;
        redis)
            check_redis
            ;;
        minio)
            check_minio
            ;;
        all)
            check_postgres
            echo ""
            check_redis
            echo ""
            check_minio
            ;;
        *)
            log_error "Unknown target: $TARGET"
            echo "Usage: $0 [postgres|redis|minio|all]"
            exit 1
            ;;
    esac
    
    echo ""
    log_info "========================================"
    log_info "Health check complete"
    log_info "========================================"
}

run_full_diagnostics
