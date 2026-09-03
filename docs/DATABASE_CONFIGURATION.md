# Database Configuration Reference

## Quick Start

### Minimal Configuration (.env)
```env
# PostgreSQL Connection
DATABASE_URL=postgresql://thanawy:password@localhost:5432/thanawy?sslmode=disable

# Redis Connection (optional)
REDIS_URL=redis://127.0.0.1:6379

# MinIO Storage (optional)
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
```

### Production Configuration
```env
# PostgreSQL with SSL
DATABASE_URL=postgresql://thanawy:STRONG_PASSWORD@prod-db.example.com:5432/thanawy?sslmode=require

# Separate write endpoint (read replicas)
DATABASE_WRITE_DSN=postgresql://thanawy:STRONG_PASSWORD@prod-db-write.example.com:5432/thanawy?sslmode=require

# Read replicas
DATABASE_REPLICAS=postgresql://user:pass@replica1:5432/thanawy,postgresql://user:pass@replica2:5432/thanawy

# Enable RLS via app role
DATABASE_USE_APP_ROLE=true

# Connection pool tuning
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=25
DB_MAX_LIFETIME=15m
DB_MAX_IDLE_TIME=5m

# Redis cluster
REDIS_URL=redis://:password@redis-cluster:6379

# SSL/TLS
POSTGRES_SSL_MODE=require
REDIS_TLS=true
```

## Environment Variables Reference

### Connection Strings

| Variable | Example | Notes |
|----------|---------|-------|
| `DATABASE_URL` | `postgresql://user:pass@host:5432/db` | **Required**. Primary connection string. |
| `DATABASE_WRITE_DSN` | `postgresql://...` | Optional. Explicit write endpoint for read-write splitting. |
| `DATABASE_REPLICAS` | `pg://...@r1:5432/db,pg://...@r2:5432/db` | Comma-separated read replicas. |
| `REDIS_URL` | `redis://:password@host:6379/0` | Redis connection. Format: `redis://[user:password@]host:port[/db]` |

### Pool Configuration

| Variable | Default (Server) | Default (Serverless) | Purpose |
|----------|------------------|----------------------|---------|
| `DB_MAX_IDLE_CONNS` | 25 | 2 | Max idle connections in pool |
| `DB_MAX_OPEN_CONNS` | 50 | 5 | Max open connections in pool |
| `DB_MAX_LIFETIME` | 15m | 1m | Close connections after this duration |
| `DB_MAX_IDLE_TIME` | 5m | 30s | Close idle connections after this duration |

**Serverless detection**: Automatic for Vercel (`VERCEL=1`), AWS Lambda (`AWS_LAMBDA_FUNCTION_NAME`), or explicit (`SERVERLESS=1`)

### Security

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_USE_APP_ROLE` | false | Enable app_user role for Row-Level Security |
| `POSTGRES_SSL_MODE` | disable | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |
| `REDIS_TLS` | false | Enable TLS for Redis connections |
| `REDIS_PASSWORD` | devpassword | Redis auth password |

### Logging & Debugging

| Variable | Default | Purpose |
|----------|---------|---------|
| `DB_LOG_LEVEL` | warn | Query logging: `info`, `warn`, `error`, `silent` |
| `DB_DEBUG` | false | Enable verbose database logging (requires APP_ENV != production) |
| `LOG_LEVEL` | info | Application log level |
| `APP_ENV` | development | Application environment: `development`, `production`, `staging` |

### Migrations

| Variable | Default | Purpose |
|----------|---------|---------|
| `SKIP_AUTO_MIGRATE` | false | Skip automatic migrations on startup |
| `DATABASE_URL_DIRECT` | (none) | Direct connection for migration runner |

## Connection String Anatomy

### PostgreSQL URL Format
```
postgresql://[user[:password]@][netloc][:port][/dbname][?param=value]
```

**Components:**
- `postgresql://` — Driver protocol (required)
- `user:password@` — Authentication (colon-separated)
- `netloc` — Host name or IP
- `:port` — Port number (default: 5432)
- `/dbname` — Database name
- `?param=value` — Query parameters

### Common Query Parameters

| Parameter | Values | Purpose |
|-----------|--------|---------|
| `sslmode` | `disable`, `require`, `verify-ca`, `verify-full` | SSL certificate verification |
| `application_name` | Any string | Sets the app name in PostgreSQL |
| `search_path` | `public,schema2` | Default schema search order |
| `connect_timeout` | Seconds | Connection timeout (default: 0, unlimited) |
| `statement_timeout` | Milliseconds | Query timeout |
| `lock_timeout` | Milliseconds | Lock acquisition timeout |

### Examples

**Development (local, no SSL):**
```
postgresql://thanawy:devpassword@localhost:5432/thanawy?sslmode=disable
```

**Production (remote, SSL required):**
```
postgresql://thanawy:LONG_PASSWORD@db.prod.aws.rds.amazonaws.com:5432/thanawy?sslmode=require&application_name=api
```

**With timeouts:**
```
postgresql://user:pass@host:5432/db?sslmode=require&connect_timeout=10&statement_timeout=30000
```

**PgBouncer (connection pooler):**
```
postgresql://user:pass@pgbouncer.local:6543/db?sslmode=disable&pgbouncer=true
```

**Read-only replica:**
```
postgresql://readonly:password@replica.example.com:5432/thanawy?sslmode=require
```

## Pool Sizing Guide

### Formula
```
MaxOpenConns = (avg_request_rate * request_duration_seconds) + buffer

MaxIdleConns = MaxOpenConns / 2 (at minimum)
```

### Examples

**High-traffic API (1000 req/sec, 100ms per query):**
```
MaxOpenConns = (1000 * 0.1) + 50 = 150
MaxIdleConns = 75
MaxLifetime = 15m
MaxIdleTime = 5m
```

**Serverless function (Vercel/Lambda):**
```
MaxOpenConns = 5        # Each instance is ephemeral
MaxIdleConns = 2        # Release connections quickly
MaxLifetime = 1m        # Force quick reset
MaxIdleTime = 30s       # Don't hold during freeze
```

**PgBouncer through (connection pooler already exists):**
```
MaxOpenConns = 20       # Pooler handles queuing
MaxIdleConns = 10
MaxLifetime = 5m        # Let pooler manage
MaxIdleTime = 1m
```

## SSL/TLS Configuration

### Development (No SSL)
```env
POSTGRES_SSL_MODE=disable
```

### Production (SSL Required)
```env
POSTGRES_SSL_MODE=require
# Optional: verify server certificate
# POSTGRES_SSL_MODE=verify-full
```

### Custom CA Certificate
```env
POSTGRES_SSL_MODE=verify-ca
DATABASE_URL=postgresql://...?sslmode=verify-ca&sslrootcert=/path/to/ca.crt
```

## Replica Configuration

### Read-Write Splitting

Primary use case: High-traffic read-heavy applications.

```env
# Write source (master)
DATABASE_URL=postgresql://user:pass@master.db:5432/thanawy

# Multiple read replicas
DATABASE_REPLICAS=postgresql://user:pass@replica1:5432/thanawy,postgresql://user:pass@replica2:5432/thanawy
```

**Code usage:**
```go
// Read from replica (randomly selected)
results, err := db.ReadDB(ctx).Where("status = ?", "active").Find(&items).Error

// Write to master
err := db.WriteDB(ctx).Create(&item).Error

// Transaction (uses write source)
err := db.WithWriteTx(func(tx *gorm.DB) error {
    // All operations go to master
}, ctx)
```

## Row-Level Security (RLS) Setup

### Enable RLS
```bash
# In .env
DATABASE_USE_APP_ROLE=true

# Requires PostgreSQL user setup
```

### SQL Setup
```sql
-- Create app role for RLS
CREATE ROLE app_user WITH LOGIN ENCRYPTED PASSWORD 'STRONG_PASSWORD';
GRANT CONNECT ON DATABASE thanawy TO app_user;
GRANT USAGE ON SCHEMA public TO app_user;

-- Grant table permissions
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;

-- Enable RLS policies on tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY user_isolation ON users USING (user_id = current_user_id());
```

## Troubleshooting Configuration Issues

### "Connection refused"
**Check:**
- Is PostgreSQL running? `docker compose ps postgres`
- Correct port? Default 5432
- Firewall allows connection?
- Host name resolves? `nslookup hostname`

**Fix:**
```bash
# Test connection
docker compose exec postgres pg_isready -U thanawy -h postgres

# Verify DSN
docker compose config | grep DATABASE_URL
```

### "FATAL: password authentication failed"
**Check:**
- Credentials in DATABASE_URL match PostgreSQL container
- POSTGRES_PASSWORD matches password in URL

**Fix:**
```bash
# Reset password in PostgreSQL
docker compose exec postgres psql -U postgres -d postgres -c \
  "ALTER ROLE thanawy WITH PASSWORD 'newpassword';"

# Update .env
DATABASE_URL=postgresql://thanawy:newpassword@localhost:5432/thanawy
```

### "SSL mode is not enabled"
**For production:**
```env
POSTGRES_SSL_MODE=require
# In DATABASE_URL, add: ?sslmode=require
```

### "Too many connections"
**Check pool settings:**
```bash
# View current limits
docker compose exec postgres psql -U thanawy -d thanawy -c "SHOW max_connections;"

# View active connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT count(*) FROM pg_stat_activity;"
```

**Fix:**
```env
# Reduce pool size
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
```

## Docker Compose Override

Create `docker-compose.override.yml` for local customization:

```yaml
services:
  postgres:
    environment:
      POSTGRES_PASSWORD: local_dev_password
      POSTGRES_INITDB_ARGS: "-c max_connections=200 -c shared_buffers=256MB"
    ports:
      - "5433:5432"  # Alternative port
    volumes:
      - ./db_backup.sql:/docker-entrypoint-initdb.d/backup.sql
```

## Performance Tuning

### PostgreSQL Configuration
```sql
-- View current settings
SHOW max_connections;
SHOW shared_buffers;
SHOW effective_cache_size;

-- Temporary changes (session)
SET work_mem TO '256MB';
SET random_page_cost TO 1.1;  -- For SSD
```

### GORM Query Optimization
```go
// Prepare statements (default ON, except PgBouncer)
db.Session(&gorm.Session{PrepareStmt: true})

// Use raw SQL for complex queries
db.Raw("SELECT * FROM table WHERE condition = ?", value).Scan(&results)

// Index columns effectively
// SELECT * WHERE indexed_column = ? (uses index)
// SELECT * WHERE indexed_column LIKE 'prefix%' (uses index)
// SELECT * WHERE indexed_column LIKE '%suffix' (full scan)
```

## References

- [PostgreSQL Connection Strings](https://www.postgresql.org/docs/current/libpq-connect.html)
- [GORM Documentation](https://gorm.io/)
- [PgBouncer Configuration](https://www.pgbouncer.org/)
- [AWS RDS PostgreSQL](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_PostgreSQL.html)
