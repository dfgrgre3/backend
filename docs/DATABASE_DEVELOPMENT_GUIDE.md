# Comprehensive Database Development & Fixing Guide

## Table of Contents
1. [Connection & Pool Configuration](#connection--pool-configuration)
2. [Migration System](#migration-system)
3. [Performance Optimization](#performance-optimization)
4. [Security Best Practices](#security-best-practices)
5. [Troubleshooting](#troubleshooting)
6. [Docker Setup](#docker-setup)
7. [Development Workflow](#development-workflow)

---

## Connection & Pool Configuration

### Database Connection Architecture
The Thanawy backend implements a sophisticated connection management system:

- **Multi-tier Connection Management**: Read replicas, write sources, and raw connections
- **CQRS Pattern**: `ReadDB()` for queries, `WriteDB()` for commands
- **Role-Based Access Control**: `app_user` role for Row-Level Security (RLS)
- **Serverless Awareness**: Adaptive pool sizing for Vercel, AWS Lambda

### Pool Settings by Environment

#### Traditional Server (Always-On)
```
MaxIdleConns:  25
MaxOpenConns:  50
MaxLifetime:   15 minutes
MaxIdleTime:   5 minutes
```

#### Serverless (Vercel/Lambda)
```
MaxIdleConns:  2
MaxOpenConns:  5
MaxLifetime:   1 minute
MaxIdleTime:   30 seconds
```

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | Required | Primary read-write connection |
| `DATABASE_WRITE_DSN` | (empty) | Explicit write endpoint (optional) |
| `DATABASE_REPLICAS` | (empty) | Comma-separated replica DSNs |
| `DATABASE_USE_APP_ROLE` | false | Enable Row-Level Security via app_user |
| `DB_MAX_IDLE_CONNS` | Auto | Override idle connections |
| `DB_MAX_OPEN_CONNS` | Auto | Override open connections |
| `DB_MAX_LIFETIME` | Auto | Override connection lifetime (e.g., "15m") |
| `DB_MAX_IDLE_TIME` | Auto | Override idle timeout (e.g., "5m") |
| `DB_LOG_LEVEL` | Warn | Set to "info" for SQL query logs |
| `DB_DEBUG` | false | Enable debug logging (requires `APP_ENV != production`) |

### Database URL Format
```
postgresql://username:password@host:5432/database?sslmode=disable&application_name=thanawy_app
```

### SSL/TLS Configuration
```
# Production (with SSL)
postgresql://user:pass@host:5432/db?sslmode=require

# Development (no SSL)
postgresql://user:pass@host:5432/db?sslmode=disable

# PgBouncer compatibility
postgresql://user:pass@host:6543/db?sslmode=disable&pgbouncer=true
```

---

## Migration System

### Migration Architecture

Thanawy uses **versioned SQL migrations** (no ORM code generation):
- Migrations live in `internal/infrastructure/database/migration/migrations/`
- Each migration file is named: `NNNN_description.sql`
- Migrations are tracked in the `schema_migrations` table
- **Advisory locks** prevent concurrent migration runners from racing

### Migration Workflow

#### 1. Creating a New Migration

```bash
# Create migration file
touch internal/infrastructure/database/migration/migrations/0045_your_change.sql
```

**Migration template:**
```sql
-- 0045_your_change.sql
-- Description: Brief change summary

-- Up migration
ALTER TABLE table_name ADD COLUMN new_column TEXT NOT NULL DEFAULT 'value';
CREATE INDEX idx_table_column ON table_name (new_column);

-- Verify
SELECT COUNT(*) FROM table_name WHERE new_column IS NULL;
```

#### 2. Running Migrations in Development

```bash
# Via Docker Compose
docker compose run migrate up

# Via direct binary
./bin/migrate up

# Check migration status
./bin/check-migration-status
```

#### 3. Migration Troubleshooting

**Checksum mismatch (migration file edited after apply):**
```bash
./bin/fix-migration-checksum <migration_id>
```

**Rollback failed migration:**
```bash
./bin/cleanup-failed-migration <migration_id>
```

**View applied migrations:**
```bash
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT id, checksum, appliedAt FROM schema_migrations ORDER BY appliedAt DESC LIMIT 10;"
```

### Migration Files Reference

| File | Purpose |
|------|---------|
| `0000_baseline_schema.sql` | Initial database schema (~220KB) |
| `0001_add_user_session.sql` | User session table |
| `0010_add_search_vector.sql` | Full-text search vectors |
| `0021_add_missing_tables.sql` | Missing domain tables |
| `0022_fix_notification_table.sql` | Notification system fixes |
| `0032_fix_event_table_columns.sql` | Event table corrections |
| `0033_fix_materialized_views.sql` | Materialized view optimization |
| `0043_database_health_hardening.sql` | Health check infrastructure |
| `0044_safe_database_optimization.sql` | Performance optimizations |

---

## Performance Optimization

### Query Patterns

#### Use Read Replicas (for read-heavy queries)
```go
results, err := db.ReadDB(ctx).
    Where("status = ?", "active").
    Find(&items).Error
```

#### Use Write Source (for mutations)
```go
err := db.WriteDB(ctx).
    Model(&item).
    Update("status", "completed").Error
```

#### Use Transactions for Multi-Step Operations
```go
err := db.WithWriteTx(func(tx *gorm.DB) error {
    if err := tx.Create(&newItem).Error; err != nil {
        return err
    }
    return tx.Model(&parent).Update("item_count", gorm.Expr("item_count + 1")).Error
}, ctx)
```

### Indexes

**Check existing indexes:**
```sql
SELECT indexname, indexdef 
FROM pg_indexes 
WHERE schemaname = 'public' 
ORDER BY indexname;
```

**Create index concurrently (for large tables):**
```sql
CREATE INDEX CONCURRENTLY idx_table_column ON table_name (column);
```

**Analyze table statistics:**
```sql
ANALYZE table_name;
```

### Connection Pool Monitoring

```sql
-- Current connections
SELECT datname, usename, count(*) 
FROM pg_stat_activity 
GROUP BY datname, usename;

-- Idle connections
SELECT pid, usename, query, state 
FROM pg_stat_activity 
WHERE state = 'idle';

-- Long-running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query 
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
```

---

## Security Best Practices

### Row-Level Security (RLS)

Enable RLS with the app role:
```bash
DATABASE_USE_APP_ROLE=true docker compose up
```

This creates a separate `rawWriteDB` connection that bypasses RLS for internal telemetry only.

### Connection String Security

✅ **DO**:
- Store `DATABASE_URL` in secure environment variables
- Use strong passwords (minimum 32 characters)
- Enable SSL in production (`sslmode=require`)
- Rotate credentials regularly

❌ **DON'T**:
- Commit credentials to version control
- Use default passwords (change `change_me_strong_password`)
- Log connection strings in debug output
- Store credentials in Docker images

### PostgreSQL User Roles

**Recommended role hierarchy:**

```sql
-- Schema change role (migrations)
CREATE ROLE migration_user WITH LOGIN ENCRYPTED PASSWORD 'strong_pass';
GRANT CREATE ON DATABASE thanawy TO migration_user;

-- Application role (RLS-enforced)
CREATE ROLE app_user WITH LOGIN ENCRYPTED PASSWORD 'strong_pass';
GRANT CONNECT ON DATABASE thanawy TO app_user;
GRANT USAGE ON SCHEMA public TO app_user;

-- Read-only role (reporting)
CREATE ROLE read_only_user WITH LOGIN ENCRYPTED PASSWORD 'strong_pass';
GRANT CONNECT ON DATABASE thanawy TO read_only_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO read_only_user;
```

---

## Troubleshooting

### Connection Errors

**Error: "connection refused"**
```bash
# Check PostgreSQL is running
docker compose ps postgres

# Check port mappings
docker compose port postgres 5432

# Test connection
docker compose exec postgres psql -U thanawy -d thanawy -c "SELECT 1;"
```

**Error: "FATAL: password authentication failed"**
- Verify credentials in `.env` match PostgreSQL container
- Check `POSTGRES_USER` and `POSTGRES_PASSWORD` environment variables

**Error: "SSL mode is not enabled"**
```
# Development (SSL not required)
sslmode=disable

# Production (SSL required)
sslmode=require
```

### Performance Issues

**Slow queries:**
```bash
# Enable query logging
DB_LOG_LEVEL=info docker compose up backend

# Monitor slow query log
docker compose logs -f backend | grep "took"
```

**High connection count:**
```sql
-- Check connection limits
SHOW max_connections;

-- Terminate idle connections
SELECT pg_terminate_backend(pid) 
FROM pg_stat_activity 
WHERE state = 'idle' 
AND query_start < now() - interval '10 minutes';
```

**Out of memory:**
```sql
-- Check cache size
SHOW shared_buffers;

-- Reduce cache:
SHOW work_mem;
```

### Migration Issues

**Migration stuck:**
```bash
# Check lock status
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT * FROM pg_locks WHERE granted = false;"

# Clear advisory lock
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_advisory_unlock(hashtext('thanawy_backend_schema_migrations'));"
```

**Checksum mismatch:**
```bash
# Regenerate checksum
./bin/fix-migration-checksum 0045_your_migration

# Or drop and reapply
./bin/cleanup-failed-migration 0045_your_migration
docker compose run migrate up
```

---

## Docker Setup

### docker-compose.yml Services

**PostgreSQL**
- Image: `postgres:17.6-alpine`
- Port: `5432` (mapped to host)
- Volume: `postgres_data:/var/lib/postgresql/data`
- Healthcheck: `pg_isready` (10s interval)

**Database Configuration**
```yaml
environment:
  POSTGRES_USER: thanawy          # Default: thanawy
  POSTGRES_PASSWORD: change_me    # Change in production!
  POSTGRES_DB: thanawy            # Database name
```

### Starting Services

```bash
# Full stack with migrations
docker compose up -d

# Only database
docker compose up -d postgres

# With development tools (Mailpit, etc.)
docker compose --profile dev up -d

# Initialize MinIO buckets
docker compose --profile init run minio-init

# Check service health
docker compose ps
```

### Database Operations in Docker

```bash
# Connect to PostgreSQL
docker compose exec postgres psql -U thanawy -d thanawy

# View logs
docker compose logs postgres

# Run migrations
docker compose run migrate up

# Backup database
docker compose exec postgres pg_dump -U thanawy thanawy > backup.sql

# Restore database
docker compose exec -T postgres psql -U thanawy thanawy < backup.sql

# Execute SQL command
docker compose exec postgres psql -U thanawy -d thanawy -c "SELECT * FROM schema_migrations;"
```

---

## Development Workflow

### Local Development Setup

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Update .env with your settings
POSTGRES_PASSWORD=your_strong_password
DATABASE_URL=postgresql://thanawy:your_strong_password@localhost:5432/thanawy

# 3. Start database
docker compose up -d postgres

# 4. Wait for healthcheck
sleep 10

# 5. Run migrations
docker compose run migrate up

# 6. Verify schema
docker compose exec postgres psql -U thanawy -d thanawy -c "SELECT * FROM schema_migrations;"

# 7. Start application
docker compose up backend
```

### Common Development Tasks

**Add a new table:**
1. Create migration: `touch internal/infrastructure/database/migration/migrations/NNNN_add_table.sql`
2. Write SQL in migration
3. Apply: `docker compose run migrate up`
4. Test: `docker compose exec postgres psql -U thanawy -d thanawy -c "SELECT * FROM new_table;"`

**Modify an existing table:**
1. Create migration for the change
2. Use `ALTER TABLE` for backward-compatible changes
3. Test on local database first
4. Verify no locked tables: `SELECT * FROM pg_locks;`

**Debug connection issues:**
```bash
# Check environment
docker compose config | grep DATABASE

# Test connection from app
docker compose exec backend ./main

# View logs
docker compose logs backend --tail 100 -f
```

### Performance Profiling

```bash
# Enable query logging
DB_LOG_LEVEL=info docker compose up

# Analyze slow queries
docker compose logs backend | grep "took" | sort -t= -k2 -rn | head -20

# Generate query plan
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "EXPLAIN ANALYZE SELECT * FROM table_name WHERE condition;"
```

---

## Advanced Topics

### Read Replicas

Setup read-only replica:
```bash
DATABASE_REPLICAS="postgresql://user:pass@replica1:5432/db,postgresql://user:pass@replica2:5432/db" \
docker compose up
```

### PgBouncer Connection Pooling

For high-concurrency environments:
```
# Use PgBouncer URL format
postgresql://user:pass@pgbouncer:6543/db?sslmode=disable&pgbouncer=true
```

### Monitoring & Observability

- **Prometheus metrics**: Exposed at `/metrics`
- **Sentry error tracking**: Configure `SENTRY_DSN`
- **Query logging**: Set `DB_LOG_LEVEL=info`
- **Health check**: `GET /health`

---

## References

- PostgreSQL Documentation: https://www.postgresql.org/docs/
- GORM Guide: https://gorm.io/docs/
- Docker Compose: https://docs.docker.com/compose/
- CQRS Pattern: https://martinfowler.com/bliki/CQRS.html

