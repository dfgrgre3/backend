# Database Development & Fixing - Comprehensive Package

This package contains everything needed for professional database development, maintenance, and troubleshooting in the Thanawy backend.

## 📦 What's Included

### Documentation

1. **DATABASE_DEVELOPMENT_GUIDE.md** — Complete guide covering:
   - Connection and pool configuration
   - Migration system architecture and workflow
   - Performance optimization techniques
   - Security best practices
   - Docker setup and operations
   - Development workflow

2. **DATABASE_CONFIGURATION.md** — Detailed reference for:
   - Quick start configurations
   - Environment variables reference
   - Connection string anatomy
   - Pool sizing formulas
   - SSL/TLS setup
   - Replica configuration
   - Row-Level Security (RLS) setup
   - Troubleshooting configuration issues

3. **DATABASE_TROUBLESHOOTING.md** — Step-by-step solutions for:
   - Connection errors
   - Migration issues
   - Performance problems
   - Data integrity issues
   - Backup and recovery
   - Redis troubleshooting

### Scripts

1. **scripts/db-health-check.sh** — Database diagnostics
   - Check PostgreSQL, Redis, MinIO health
   - Monitor connections and resource usage
   - Display migration status

2. **scripts/db-migrations.sh** — Migration management
   - Show migration status
   - Apply pending migrations
   - Create new migrations
   - Validate migration files
   - Check migration system health

3. **scripts/db-optimize.sh** — Performance optimization
   - Analyze tables and indexes
   - Run VACUUM operations
   - Check for table bloat
   - Monitor connections
   - Measure cache hit ratios

### Configuration Files

1. **.env.database.template** — Environment variable template
   - All configurable parameters documented
   - Production best practices
   - Development quick start

### Makefile Additions

New database management targets:
```bash
make db-health      # Check database health
make db-migrate     # Run migrations
make db-status      # Show migration status
make db-optimize    # Optimize database
make db-backup      # Backup to SQL file
make db-shell       # Open PostgreSQL shell
```

---

## 🚀 Quick Start

### 1. Setup Development Environment

```bash
# Copy environment template
cp .env.database.template .env

# Customize for your setup
# (usually just change POSTGRES_PASSWORD)

# Start all services
docker compose up -d

# Wait for PostgreSQL to be ready
sleep 10

# Check health
make db-health

# Apply migrations
make db-migrate

# Verify migrations
make db-status
```

### 2. Daily Development Tasks

```bash
# Check if everything is working
make db-health

# View recent logs
docker compose logs postgres --tail 50

# Open database shell
make db-shell

# Monitor performance
make db-optimize
```

### 3. Create a New Migration

```bash
# Create migration file
./scripts/db-migrations.sh create your_change_name

# Edit the generated file
# Add your SQL in the migration file

# Apply migration
make db-migrate

# Verify it worked
make db-status
```

---

## 🔧 Common Tasks

### Database Connection Issues

```bash
# Diagnose problem
make db-health

# Check container status
docker compose ps postgres

# View detailed logs
docker compose logs postgres | tail -100

# Test connection directly
docker compose exec postgres pg_isready -U thanawy
```

### Migration Stuck or Failed

```bash
# Check status
make db-status

# View error details
docker compose logs migrate

# Clear stuck lock (if needed)
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT pg_advisory_unlock_all();"

# Retry
make db-migrate
```

### Performance Optimization

```bash
# Run full optimization
make db-optimize

# Analyze specific tables
docker compose exec postgres psql -U thanawy -d thanawy << EOF
SELECT * FROM pg_stat_user_tables ORDER BY seq_scan DESC LIMIT 10;
EOF

# Check index usage
make db-optimize  # Includes index analysis
```

### Backup and Recovery

```bash
# Backup database
make db-backup

# List backups
ls -lh ./backups/

# Restore from backup
docker compose exec -T postgres psql -U thanawy thanawy < ./backups/backup_20240101_120000.sql
```

---

## 📊 Architecture Overview

### Connection Management
```
┌─────────────────────────────────────────────┐
│         GORM Database Layer                 │
├─────────────────────────────────────────────┤
│  ReadDB()    │ WriteDB()  │ RawWriteDB()   │
│  (Replicas)  │ (Primary)  │ (No RLS)       │
├─────────────────────────────────────────────┤
│         Connection Pool (Adaptive)          │
│    [Serverless: 2-5] [Server: 25-50]      │
├─────────────────────────────────────────────┤
│  PostgreSQL + Replicas + Redis + MinIO    │
└─────────────────────────────────────────────┘
```

### Migration System
```
Migration Files (*.sql)
        ↓
   Migration Runner
   (cmd/migrate)
        ↓
   Advisory Lock
   (Prevent races)
        ↓
   schema_migrations Table
   (Track applied migrations)
        ↓
   PostgreSQL Database
```

---

## 🔐 Security Best Practices

### Development
- Use strong local passwords (minimum 16 chars)
- Store credentials in `.env` (never in .git)
- Disable SSL for local connections

### Production
- Use strong passwords (minimum 32 characters)
- Enable SSL (`sslmode=require`)
- Use managed database services (RDS, DigitalOcean)
- Enable automated backups and point-in-time recovery
- Enable Row-Level Security (RLS) for multi-tenant isolation
- Use secrets manager (Vault, AWS Secrets Manager)
- Monitor and audit database access

---

## 📈 Performance Tips

### Connection Pool Tuning
```
# High-traffic API (1000 req/sec)
MaxOpenConns = 150
MaxIdleConns = 75

# Serverless (Vercel/Lambda)
MaxOpenConns = 5
MaxIdleConns = 2

# PgBouncer (connection pooler)
MaxOpenConns = 20
MaxIdleConns = 10
```

### Query Optimization
```go
// Use read replicas for queries
db.ReadDB(ctx).Where(...).Find(&items)

// Use transactions for multi-step ops
db.WithWriteTx(func(tx *gorm.DB) error {
    // Multiple operations
})

// Create proper indexes
// SELECT * WHERE indexed_column = ?  (FAST)
// SELECT * WHERE LIKE 'prefix%'      (FAST)
// SELECT * WHERE LIKE '%suffix'      (SLOW - full scan)
```

---

## 🐛 Troubleshooting Guide

See **DATABASE_TROUBLESHOOTING.md** for detailed solutions:

1. **Connection Issues** — "connection refused", "password auth failed"
2. **Migration Issues** — "lock held", "checksum mismatch", "syntax error"
3. **Performance Issues** — slow queries, high connections, memory exhaustion
4. **Data Integrity** — orphaned rows, corruption, constraint violations

### Quick Diagnostic Commands

```bash
# Check all services
make db-health

# Show active connections
docker compose exec postgres psql -U thanawy -d thanawy -c \
  "SELECT * FROM pg_stat_activity WHERE datname = 'thanawy';"

# Check slow queries
DB_LOG_LEVEL=info docker compose up backend
docker compose logs backend | grep "took"

# View migrations
make db-status

# Monitor memory/CPU
docker stats postgres redis
```

---

## 📚 File Reference

| File | Purpose |
|------|---------|
| `docs/DATABASE_DEVELOPMENT_GUIDE.md` | Complete development guide |
| `docs/DATABASE_CONFIGURATION.md` | Configuration reference |
| `docs/DATABASE_TROUBLESHOOTING.md` | Problem solving guide |
| `.env.database.template` | Environment variables template |
| `scripts/db-health-check.sh` | Health diagnostics |
| `scripts/db-migrations.sh` | Migration management |
| `scripts/db-optimize.sh` | Performance optimization |
| `Makefile` | Database targets (make db-*) |

---

## 🔗 Related Resources

### Internal Modules
- `internal/infrastructure/database/db.go` — Main connection logic
- `internal/infrastructure/database/db_pool_config.go` — Pool configuration
- `internal/infrastructure/database/db_sessions.go` — Session helpers
- `internal/infrastructure/database/migration/` — Migration system

### External Documentation
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [GORM Guide](https://gorm.io/docs/)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Row-Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)

---

## 🎯 Next Steps

1. **Read** — Start with `DATABASE_DEVELOPMENT_GUIDE.md`
2. **Setup** — Follow the Quick Start section above
3. **Practice** — Create a test migration with `./scripts/db-migrations.sh create test`
4. **Reference** — Keep `DATABASE_CONFIGURATION.md` handy for config questions
5. **Troubleshoot** — Use `DATABASE_TROUBLESHOOTING.md` when issues arise
6. **Automate** — Add `make db-*` commands to CI/CD pipelines

---

## 💬 Questions?

Check these in order:
1. **Configuration issues** → `DATABASE_CONFIGURATION.md`
2. **How-to questions** → `DATABASE_DEVELOPMENT_GUIDE.md`
3. **Problems/errors** → `DATABASE_TROUBLESHOOTING.md`
4. **Quick reference** → Makefile targets (`make help`)
5. **Health checks** → `make db-health`

---

## ✅ Checklist

Before deploying to production:

- [ ] Read security best practices section
- [ ] Enable SSL/TLS (`sslmode=require`)
- [ ] Change all default passwords
- [ ] Set up automated backups
- [ ] Enable monitoring (connection counts, slow queries)
- [ ] Test recovery procedure
- [ ] Enable Row-Level Security if multi-tenant
- [ ] Review connection pool settings for your load
- [ ] Set up proper logging and alerting
- [ ] Document any custom configuration

