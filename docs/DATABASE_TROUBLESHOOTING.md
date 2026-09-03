# Database Troubleshooting Guide

## Quick Diagnostics

### Run Full Health Check
```bash
# Check all services
./scripts/db-health-check.sh all

# Check specific service
./scripts/db-health-check.sh postgres
./scripts/db-health-check.sh redis
./scripts/db-health-check.sh minio
```

### Check Container Status
```bash
# View all services
docker compose ps

# View logs
docker compose logs postgres --tail 50 -f

# Check specific container
docker compose inspect postgres
```

---

## Connection Issues

### Error: "Connection refused"

**Symptoms:**
- Backend cannot start
- Logs show: `connection refused` or `dial tcp: connect: connection refused`
- `docker compose ps` shows postgres not running

**Diagnosis:**
```bash
# 1. Check if container is running
docker compose ps postgres

# 2. Check port mappings
docker compose port postgres 5432

# 3. Check container logs
docker compose logs postgres

# 4. Test connection from host
psql postgresql://thanawy:[REDACTED]@localhost:5432/thanawy

# 5. Test from another container
docker compose exec backend psql -h postgres -U thanawy -d thanawy -c "SELECT 1;"
```

**Common Causes & Fixes:**

| Issue | Fix |
|-------|-----|
| Container not running | `docker compose up -d postgres` |
| Port already in use | Change `POSTGRES_PORT` in `.env` |
| Network issues | `docker compose down -v && docker compose up -d` |
| Wrong credentials | Verify `.env`: `POSTGRES_USER`, `POSTGRES_PASSWORD` |
| Hostname not resolved | Use `postgres` (service name) not `localhost` from containers |

**Fix Steps:**
```bash
# 1. Restart PostgreSQL
docker compose restart postgres

# 2. Wait for healthcheck
sleep 15

# 3. Verify connection
docker compose exec -T postgres pg_isready -U thanawy

# 4. If still failing, check logs
docker compose logs postgres | head -100
```

### Error: "FATAL: password authentication failed"

**Symptoms:**
- Logs show: `FATAL: password authentication failed for user "thanawy"`
- Cannot connect even with correct credentials

**Diagnosis:**
```bash
# Check environment
docker compose config | grep -E "POSTGRES_USER|POSTGRES_PASSWORD"

# Verify the password matches DATABASE_URL
grep DATABASE_URL .env | head -1
```

**Causes & Fixes:**

| Cause | Fix |
|-------|-----|
| Password doesn't match | Update `.env`: `POSTGRES_PASSWORD=newpassword` |
| User doesn't exist | Check `POSTGRES_USER` matches connection string |
| Environment not loaded | Ensure `.env` exists and is readable |
| Password changed mid-run | Restart: `docker compose down && docker compose up -d` |

**Recovery:**
```bash
# Reset password directly
docker compose exec postgres psql -U postgres -d postgres << EOF
ALTER ROLE thanawy WITH PASSWORD 'new_secure_password';
EOF

# Update .env
echo 'POSTGRES_PASSWORD=new_secure_password' >> .env

# Restart
docker compose restart postgres backend
```

### Error: "host is not allowed to connect"

**Symptoms:**
- Logs show: `host is not allowed to connect` or `no pg_hba.conf entry`
- Authentication succeeds but connection fails

**Diagnosis:**
```bash
# Check pg_hba.conf inside container
docker compose exec postgres cat /var/lib/postgresql/data/pg_hba.conf | head -20
```

**Fix:**
```bash
# This is usually a PostgreSQL container configuration issue
# Recreate without persistent volume:
docker compose down -v
docker compose up -d postgres

# Or use a custom pg_hba.conf
docker compose exec postgres psql -U postgres -c \
  "SHOW hba_file;"
```

---

## Migration Issues

### Error: "migration lock is held"

**Symptoms:**
- Migrations stuck with: `acquire migration lock: context deadline exceeded`
- Running migrations from multiple instances simultaneously

**Diagnosis:**
```bash
# Check advisory lock
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pid, pg_blocking_pids(pid), query FROM pg_stat_activity WHERE pid != pg_backend_pid();"

# Check lock holder
docker compose logs migrate --tail 20
```

**Fix:**
```bash
# 1. Kill stuck migration process
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE '%migration%';"

# 2. Clear advisory lock
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_advisory_unlock_all();"

# 3. Retry migrations
docker compose run --rm migrate up
```

### Error: "Checksum mismatch"

**Symptoms:**
- Logs show: `checksum mismatch for migration 0045_...`
- Migration file was edited after being applied

**Diagnosis:**
```bash
# View migration records
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT id, checksum FROM schema_migrations WHERE id = '0045_...';"

# Compare with file
sha256sum internal/infrastructure/database/migration/migrations/0045_*.sql
```

**Fix:**
```bash
# 1. Recompute checksum (if file is correct)
./bin/fix-migration-checksum 0045_your_migration

# 2. Or rollback and reapply (if file was wrong)
./bin/cleanup-failed-migration 0045_your_migration
docker compose run --rm migrate up

# 3. Or manually update (not recommended)
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "UPDATE schema_migrations SET checksum = 'new_checksum' WHERE id = '0045_...';"
```

### Error: "syntax error in migration SQL"

**Symptoms:**
- Migration fails with: `ERROR: syntax error at or near ...`
- Table creation fails

**Diagnosis:**
```bash
# View the migration file
cat internal/infrastructure/database/migration/migrations/0045_*.sql

# Test SQL directly
docker compose exec postgres psql -U thanawy -d thanawy << EOF
-- Paste SQL from migration here
EOF
```

**Fix:**
```bash
# 1. Check SQL syntax
# Common errors:
# - Missing semicolon at statement end
# - Typo in table/column name
# - Invalid data type

# 2. After fixing, reapply
docker compose run --rm migrate up

# 3. If still failing, cleanup and retry
./bin/cleanup-failed-migration 0045_your_migration
docker compose run --rm migrate up
```

---

## Performance Issues

### Slow Queries

**Diagnosis:**
```bash
# Enable query logging
DB_LOG_LEVEL=info docker compose up backend

# Watch for slow queries
docker compose logs -f backend | grep "took"

# Analyze query plan
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "EXPLAIN ANALYZE SELECT * FROM large_table WHERE condition = 'value';"
```

**Optimization:**
```bash
# Run maintenance
./scripts/db-optimize.sh all

# Check index usage
./scripts/db-optimize.sh indexes

# Analyze table statistics
./scripts/db-optimize.sh analyze
```

### High Connection Count

**Symptoms:**
- Logs show: `too many connections`
- Database becoming unresponsive
- Connection pool exhausted

**Diagnosis:**
```bash
# Check active connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT datname, usename, state, count(*) FROM pg_stat_activity GROUP BY datname, usename, state;"

# Check connection limit
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SHOW max_connections;"
```

**Fix:**
```bash
# 1. Reduce pool size
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10

# 2. Kill idle connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle' AND query_start < now() - interval '10 minutes';"

# 3. Restart backend
docker compose restart backend
```

### Memory Exhaustion

**Symptoms:**
- PostgreSQL container stops or becomes unresponsive
- Docker logs show OOM kills
- Queries fail with `out of memory`

**Diagnosis:**
```bash
# Check memory usage
docker stats postgres

# Check PostgreSQL memory settings
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT name, setting, unit FROM pg_settings WHERE name ~ 'memory|buffers';"
```

**Fix:**
```bash
# Increase container memory
# In docker-compose.yml:
# services:
#   postgres:
#     deploy:
#       resources:
#         limits:
#           memory: 2G

# Or via environment
docker run -m 2G -e POSTGRES_INITDB_ARGS="-c shared_buffers=512MB" postgres:17.6-alpine
```

---

## Data Integrity

### Orphaned or Corrupted Rows

**Symptoms:**
- Foreign key constraint violations
- Duplicate key errors
- NULL in NOT NULL columns

**Diagnosis:**
```bash
# Check foreign key violations
docker compose exec postgres psql -U thanawy -d thanawy << EOF
SELECT * FROM table_a
WHERE fk_id NOT IN (SELECT id FROM table_b);
EOF

# Check duplicates
docker compose exec postgres psql -U thanawy -d thanawy << EOF
SELECT column, COUNT(*) FROM table
GROUP BY column HAVING COUNT(*) > 1;
EOF
```

**Fix:**
```bash
# Backup first
docker compose exec postgres pg_dump -U thanawy thanawy > backup_$(date +%s).sql

# Fix violations
docker compose exec postgres psql -U thanawy -d thanawy << EOF
DELETE FROM table_a WHERE fk_id NOT IN (SELECT id FROM table_b);
EOF

# Verify constraints
docker compose exec postgres psql -U thanawy -d thanawy << EOF
ALTER TABLE table_a VALIDATE CONSTRAINT fk_constraint;
EOF
```

---

## Backup & Recovery

### Backup Database

```bash
# Full backup
docker compose exec -T postgres pg_dump -U thanawy thanawy > backup.sql

# Compressed backup
docker compose exec -T postgres pg_dump -U thanawy thanawy | gzip > backup.sql.gz

# With schema only
docker compose exec -T postgres pg_dump -U thanawy thanawy --schema-only > schema.sql

# With data only
docker compose exec -T postgres pg_dump -U thanawy thanawy --data-only > data.sql
```

### Restore Database

```bash
# Restore from backup
docker compose exec -T postgres psql -U thanawy thanawy < backup.sql

# Restore compressed backup
gunzip -c backup.sql.gz | docker compose exec -T postgres psql -U thanawy thanawy

# Restore to different database
docker compose exec postgres psql -U postgres -c "CREATE DATABASE thanawy_restore;"
docker compose exec -T postgres psql -U thanawy thanawy_restore < backup.sql
```

### Point-in-Time Recovery

```bash
# List WAL archives (if enabled)
docker compose exec postgres ls -lah /var/lib/postgresql/data/pg_wal/

# Recover to specific timestamp
docker compose exec postgres psql -U thanawy -d thanawy << EOF
-- PostgreSQL 14+
SELECT pg_wal_replay_pause();
SELECT pg_wal_replay_resume();
EOF
```

---

## Redis Issues

### Redis Connection Failed

**Diagnosis:**
```bash
# Check if running
docker compose ps redis

# Test connection
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" ping

# Check memory
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" INFO memory
```

**Fix:**
```bash
# Restart Redis
docker compose restart redis

# Clear cache (if safe)
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" FLUSHDB

# Check logs
docker compose logs redis --tail 50
```

### Redis Memory Full

**Diagnosis:**
```bash
# Check memory usage
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" INFO memory | grep used

# View key sizes
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" --bigkeys
```

**Fix:**
```bash
# Eviction policy (set in docker-compose.yml)
# command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru

# Or clear selectively
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" KEYS "prefix:*" | \
  xargs docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" DEL
```

---

## Support Resources

- **PostgreSQL Docs**: https://www.postgresql.org/docs/
- **GORM Issues**: https://github.com/go-gorm/gorm/issues
- **Docker Compose**: https://docs.docker.com/compose/
- **Health Check Script**: `./scripts/db-health-check.sh`
- **Optimization Script**: `./scripts/db-optimize.sh`
- **Migration Script**: `./scripts/db-migrations.sh`

