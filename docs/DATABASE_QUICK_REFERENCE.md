# Database Quick Reference Card

## Essential Commands

### Health & Diagnostics
```bash
# Full health check
make db-health
./scripts/db-health-check.sh all

# Check specific service
./scripts/db-health-check.sh postgres
./scripts/db-health-check.sh redis
./scripts/db-health-check.sh minio

# View container status
docker compose ps
```

### Database Connection
```bash
# Open PostgreSQL shell
make db-shell
docker compose exec postgres psql -U thanawy -d thanawy

# Open Redis CLI
make redis-cli
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}"

# Test connection
docker compose exec postgres pg_isready -U thanawy
docker compose exec redis redis-cli -a "${REDIS_PASSWORD:-devpassword}" ping
```

### Migrations
```bash
# Apply pending migrations
make db-migrate
docker compose run --rm migrate up

# Show migration status
make db-status
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT * FROM schema_migrations ORDER BY appliedAt DESC LIMIT 20;"

# Create new migration
./scripts/db-migrations.sh create your_change_name

# Validate migrations
./scripts/db-migrations.sh validate

# Check migration health
./scripts/db-migrations.sh health
```

### Performance & Optimization
```bash
# Full optimization
make db-optimize
./scripts/db-optimize.sh all

# Analyze tables
./scripts/db-optimize.sh analyze

# Check indexes
./scripts/db-optimize.sh indexes

# Monitor connections
./scripts/db-optimize.sh connections

# Check cache hit ratio
./scripts/db-optimize.sh cache

# Run VACUUM
./scripts/db-optimize.sh vacuum

# Check table bloat
./scripts/db-optimize.sh bloat
```

### Backup & Recovery
```bash
# Backup database
make db-backup
docker compose exec -T postgres pg_dump -U thanawy thanawy > backup.sql

# Restore from backup
docker compose exec -T postgres psql -U thanawy thanawy < backup.sql

# Compressed backup
docker compose exec -T postgres pg_dump -U thanawy thanawy | gzip > backup.sql.gz
```

### Logs
```bash
# View logs
docker compose logs postgres --tail 50
docker compose logs -f postgres

# View specific service logs
docker compose logs backend
docker compose logs redis

# Follow all logs
docker compose logs -f
```

### Container Management
```bash
# Start services
docker compose up -d
docker compose up -d postgres
docker compose up -d --profile dev

# Stop services
docker compose down

# Stop and remove data (WARNING)
docker compose down -v

# Restart services
docker compose restart postgres
docker compose restart

# View resource usage
docker stats postgres redis
```

---

## Environment Configuration

### Quick Start (.env)
```env
DATABASE_URL=postgresql://thanawy:[REDACTED]@localhost:5432/thanawy?sslmode=disable
REDIS_URL=redis://:[REDACTED]@localhost:6379
POSTGRES_PASSWORD=change_me_strong_password
APP_ENV=development
```

### Production (.env)
```env
DATABASE_URL=postgresql://thanawy:[REDACTED]@prod-db:5432/thanawy?sslmode=require
DATABASE_USE_APP_ROLE=true
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=25
APP_ENV=production
```

### Serverless (.env)
```env
DATABASE_URL=postgresql://...?sslmode=require
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
SERVERLESS=1
```

---

## Common Issues & Fixes

### Connection Refused
```bash
# Start PostgreSQL
docker compose up -d postgres
sleep 10

# Test connection
docker compose exec postgres pg_isready -U thanawy

# Check logs
docker compose logs postgres | tail -50
```

### Password Auth Failed
```bash
# Verify credentials match
docker compose config | grep -E "POSTGRES_PASSWORD|DATABASE_URL"

# Update .env
POSTGRES_PASSWORD=newpassword
DATABASE_URL=postgresql://thanawy:[REDACTED]@localhost:5432/thanawy

# Restart
docker compose restart postgres backend
```

### Migration Lock Held
```bash
# Clear lock
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_advisory_unlock_all();"

# Retry
make db-migrate
```

### Too Many Connections
```bash
# Check active connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT COUNT(*) FROM pg_stat_activity;"

# Kill idle connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle';"

# Reduce pool size in .env
DB_MAX_OPEN_CONNS=25
docker compose restart backend
```

### Slow Queries
```bash
# Enable query logging
DB_LOG_LEVEL=info docker compose up backend

# Watch for slow queries
docker compose logs -f backend | grep "took"

# Analyze query
docker compose exec postgres psql -U thanawy -d thanawy << EOF
EXPLAIN ANALYZE SELECT * FROM table WHERE condition = 'value';
EOF
```

---

## Pool Configuration Cheat Sheet

| Scenario | MaxOpen | MaxIdle | MaxLifetime | MaxIdleTime |
|----------|---------|---------|-------------|------------|
| Development | 50 | 25 | 15m | 5m |
| Production | 100-300 | 50-150 | 15m | 5m |
| Serverless | 5 | 2 | 1m | 30s |
| PgBouncer | 20 | 10 | 5m | 1m |
| High Traffic | 200+ | 100+ | 15m | 5m |
| Read-Heavy | 50 | 25 | 15m | 5m |

---

## Code Patterns

### Read (Query)
```go
results, err := db.ReadDB(ctx).
    Where("status = ?", "active").
    Find(&items).Error
```

### Write (Command)
```go
err := db.WriteDB(ctx).
    Model(&item).
    Update("status", "completed").Error
```

### Transaction
```go
err := db.WithWriteTx(func(tx *gorm.DB) error {
    if err := tx.Create(&newItem).Error; err != nil {
        return err
    }
    return tx.Model(&parent).Update("count", gorm.Expr("count + 1")).Error
}, ctx)
```

### Telemetry (Bypasses RLS)
```go
err := db.RawWriteDB(ctx).
    Create(&telemetryData).Error
```

---

## Files & Directories

```
backend/
├── docs/
│   ├── DATABASE_PACKAGE_README.md      ← START HERE
│   ├── DATABASE_DEVELOPMENT_GUIDE.md    ← Full guide
│   ├── DATABASE_CONFIGURATION.md        ← Reference
│   └── DATABASE_TROUBLESHOOTING.md      ← Problem solving
├── scripts/
│   ├── db-health-check.sh               ← Diagnostics
│   ├── db-migrations.sh                 ← Migration mgmt
│   └── db-optimize.sh                   ← Performance
├── .env.database.template               ← Config template
├── internal/infrastructure/database/
│   ├── db.go                            ← Connection logic
│   ├── db_pool_config.go                ← Pool settings
│   ├── db_sessions.go                   ← Session helpers
│   └── migration/                       ← Migration system
└── Makefile                             ← db-* targets
```

---

## Documentation Index

| Document | Purpose | When to Use |
|----------|---------|------------|
| DATABASE_PACKAGE_README.md | Overview and quick start | First time, getting started |
| DATABASE_DEVELOPMENT_GUIDE.md | Complete development guide | Learning architecture, workflow |
| DATABASE_CONFIGURATION.md | Detailed configuration | Configuring connections, pool settings |
| DATABASE_TROUBLESHOOTING.md | Problem solving | Debugging issues |
| This card | Quick reference | Daily development |

---

## Make Targets

```bash
# Database targets (new)
make db-health          # Check database health
make db-migrate         # Run migrations
make db-status          # Show migration status
make db-optimize        # Run optimization
make db-backup          # Backup to SQL file
make db-shell           # Open PostgreSQL shell

# Docker targets
make docker-up          # Start all services
make docker-down        # Stop services
make docker-ps          # Show status
make docker-logs        # Follow logs
make docker-restart     # Restart all

# App targets
make build              # Build application
make test               # Run tests
make run                # Run API server
make clean              # Clean build artifacts
```

---

## Checklist: First Time Setup

- [ ] Copy `.env.database.template` to `.env`
- [ ] Customize environment variables
- [ ] Run `docker compose up -d`
- [ ] Wait 10 seconds for PostgreSQL
- [ ] Run `make db-health` to verify
- [ ] Run `make db-migrate` to apply migrations
- [ ] Run `make db-status` to confirm
- [ ] Open `docs/DATABASE_DEVELOPMENT_GUIDE.md` for deeper learning

---

## Checklist: Before Production

- [ ] Enable SSL: `sslmode=require`
- [ ] Set strong passwords (32+ chars)
- [ ] Configure connection pool
- [ ] Enable backups
- [ ] Enable monitoring
- [ ] Set `APP_ENV=production`
- [ ] Enable RLS if multi-tenant
- [ ] Test recovery procedure
- [ ] Document any custom settings
- [ ] Setup alerts for connection exhaustion

---

## Help & Resources

```bash
# Get help with script
./scripts/db-health-check.sh --help
./scripts/db-migrations.sh --help
./scripts/db-optimize.sh --help

# View Makefile targets
make help

# Show Docker Compose services
docker compose config

# Check PostgreSQL version
docker compose exec postgres psql -V

# PostgreSQL docs
# https://www.postgresql.org/docs/

# GORM docs
# https://gorm.io/docs/
```

---

**Last Updated: 2026-09-03**  
**For latest changes, see: docs/DATABASE_DEVELOPMENT_GUIDE.md**
