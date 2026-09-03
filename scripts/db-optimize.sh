#!/bin/bash
# Database optimization and maintenance script
# Usage: ./scripts/db-optimize.sh [analyze|vacuum|indexes|bloat|connections|all]

set -e

COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"
ACTION="${1:-all}"

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

# Analyze tables for query planner
optimize_analyze() {
    log_info "Analyzing tables for query optimization..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT 
    schemaname,
    tablename,
    CASE 
        WHEN last_vacuum IS NULL THEN 'Never'
        ELSE to_char(last_vacuum, 'YYYY-MM-DD HH24:MI:SS')
    END as last_vacuum,
    CASE 
        WHEN last_autovacuum IS NULL THEN 'Never'
        ELSE to_char(last_autovacuum, 'YYYY-MM-DD HH24:MI:SS')
    END as last_autovacuum
FROM pg_stat_user_tables
ORDER BY schemaname, tablename;
EOF
    
    log_info "Running ANALYZE on all tables..."
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
ANALYZE;
EOF
    
    log_success "Analysis complete"
}

# Vacuum tables
optimize_vacuum() {
    log_info "Running VACUUM on tables..."
    log_warn "This may lock tables temporarily"
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
VACUUM ANALYZE;
EOF
    
    log_success "Vacuum complete"
}

# Analyze index usage
optimize_indexes() {
    log_info "Analyzing index usage..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
-- Unused indexes
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;
EOF
    
    log_info "Index statistics..."
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
-- Most used indexes
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC
LIMIT 20;
EOF
    
    log_success "Index analysis complete"
}

# Check table bloat
optimize_bloat() {
    log_info "Checking for table bloat..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
-- Estimate table bloat (requires pgstattuple extension for accuracy)
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename, 'main')) as heap_size,
    round(100.0 * (pg_relation_size(schemaname||'.'||tablename) - 
           pg_relation_size(schemaname||'.'||tablename, 'main')) / 
           NULLIF(pg_relation_size(schemaname||'.'||tablename), 0), 2) as index_ratio
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 20;
EOF
    
    log_success "Bloat analysis complete"
}

# Monitor connections
optimize_connections() {
    log_info "Monitoring database connections..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
-- Current connections
SELECT
    datname,
    usename,
    application_name,
    state,
    COUNT(*) as connection_count
FROM pg_stat_activity
WHERE datname IS NOT NULL
GROUP BY datname, usename, application_name, state
ORDER BY connection_count DESC;
EOF
    
    log_info "Long-running queries..."
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT
    pid,
    usename,
    application_name,
    now() - query_start as duration,
    query
FROM pg_stat_activity
WHERE query NOT LIKE '%pg_stat_activity%'
  AND state != 'idle'
ORDER BY query_start
LIMIT 10;
EOF
}

# Cache hit ratio
optimize_cache() {
    log_info "Checking cache hit ratio..."
    
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT 
    sum(heap_blks_read) as heap_blks_read,
    sum(heap_blks_hit) as heap_blks_hit,
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
FROM pg_statio_user_tables;
EOF
    
    log_info "Index cache hit ratio..."
    $COMPOSE_CMD exec -T postgres psql -U thanawy -d thanawy << EOF
SELECT 
    sum(idx_blks_read) as idx_blks_read,
    sum(idx_blks_hit) as idx_blks_hit,
    sum(idx_blks_hit) / (sum(idx_blks_hit) + sum(idx_blks_read)) as ratio
FROM pg_statio_user_indexes;
EOF
}

# Full maintenance
run_full_optimization() {
    log_info "========================================"
    log_info "Database Optimization & Maintenance"
    log_info "========================================"
    echo ""
    
    case "$ACTION" in
        analyze)
            optimize_analyze
            ;;
        vacuum)
            optimize_vacuum
            ;;
        indexes)
            optimize_indexes
            ;;
        bloat)
            optimize_bloat
            ;;
        connections)
            optimize_connections
            ;;
        cache)
            optimize_cache
            ;;
        all)
            optimize_analyze
            echo ""
            optimize_indexes
            echo ""
            optimize_bloat
            echo ""
            optimize_connections
            echo ""
            optimize_cache
            ;;
        *)
            log_error "Unknown action: $ACTION"
            echo "Available actions:"
            echo "  analyze      - Analyze tables for query planning"
            echo "  vacuum       - Run VACUUM ANALYZE"
            echo "  indexes      - Analyze index usage"
            echo "  bloat        - Check for table bloat"
            echo "  connections  - Monitor connections"
            echo "  cache        - Check cache hit ratios"
            echo "  all          - Run all optimizations"
            exit 1
            ;;
    esac
    
    echo ""
    log_info "========================================"
    log_info "Optimization complete"
    log_info "========================================"
}

run_full_optimization
