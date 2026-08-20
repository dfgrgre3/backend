package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var DB *gorm.DB

// rawWriteDB is a GORM DB instance that connects without the app_user role.
// It is used for internal telemetry tables (e.g., http_metric_buckets) that
// do not require multi-tenant row isolation and must be writable even when
// the application runs with DATABASE_USE_APP_ROLE=true.
var rawWriteDB *gorm.DB

// LegacySchemaNamingStrategy preserves the existing database contract while
// decoupling GORM from the removed Prisma toolchain. New schema changes must be
// expressed as versioned SQL migrations rather than inferred from an ORM.
type LegacySchemaNamingStrategy struct {
	schema.NamingStrategy
}

func (LegacySchemaNamingStrategy) TableName(table string) string {
	return table // Model name is already PascalCase
}

// buildGormLogger returns a configured GORM logger shared by all connection helpers.
func buildGormLogger() logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  getGormLogLevel(),
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		},
	)
}

func Connect(dsn string) (*gorm.DB, error) {
	return ConnectWithWriteDSN(dsn, os.Getenv("DATABASE_WRITE_DSN"))
}

func ConnectWithWriteDSN(dsn, writeDSN string) (*gorm.DB, error) {
	// Determine the DSN for app-role connections.
	// We use DSN string manipulation (not a test connection) to avoid the
	// extra round-trip on every process start.
	useAppRole := os.Getenv("DATABASE_USE_APP_ROLE") == "true"
	appDSN := dsn

	if useAppRole {
		appDSNFromFunc, err := GetAppDSN()
		if err != nil {
			log.Printf("[WARN] Failed to build app DSN for RLS, falling back to provided DSN: %v", err)
		} else {
			appDSN = appDSNFromFunc
			log.Printf("[DB] Using app role connection to respect RLS policies")
		}
	} else {
		log.Printf("[DB] Using direct database connection (app role disabled). Set DATABASE_USE_APP_ROLE=true to enable Row Level Security.")
	}

	db, err := gorm.Open(postgres.Open(appDSN), &gorm.Config{
		Logger:         buildGormLogger(),
		PrepareStmt:    !usesPgBouncer(appDSN), // PgBouncer transaction pooling cannot safely cache prepared statements.
		NamingStrategy: LegacySchemaNamingStrategy{},
	})
	if err != nil {
		return nil, err
	}

	// Determine write source DSN.
	sourceDSN := appDSN
	if writeDSN != "" {
		// For write DSN, also apply app role to respect RLS.
		writeAppDSN, err := GetDSNForRole(writeDSN, RoleApp)
		if err != nil {
			log.Printf("[WARN] Failed to add app role to write DSN: %v", err)
			sourceDSN = writeDSN
		} else {
			sourceDSN = writeAppDSN
		}
	}

	replicaDialectors := getReplicaDialectors()
	pool := getPoolSettings()

	log.Printf("Database connection pool settings: MaxIdleConns=%d, MaxOpenConns=%d, ConnMaxLifetime=%s, ConnMaxIdleTime=%s",
		pool.MaxIdleConns, pool.MaxOpenConns, pool.MaxLifetime, pool.MaxIdleTime)

	// Bug fix: when no explicit replicas are configured, fall back to appDSN
	// (not the raw `dsn` argument) so the replica also uses the app role.
	var replicas []gorm.Dialector
	if len(replicaDialectors) > 0 {
		replicas = replicaDialectors
	} else {
		replicas = []gorm.Dialector{postgresDialector(appDSN)}
	}

	// Register DBResolver with explicit source/replica splitting for CQRS
	resolver := dbresolver.Register(dbresolver.Config{
		Sources:  []gorm.Dialector{postgresDialector(sourceDSN)},
		Replicas: replicas,
		Policy:   dbresolver.RandomPolicy{},
	}).
		SetMaxIdleConns(pool.MaxIdleConns).
		SetMaxOpenConns(pool.MaxOpenConns).
		SetConnMaxLifetime(pool.MaxLifetime).
		SetConnMaxIdleTime(pool.MaxIdleTime)

	if err := db.Use(resolver); err != nil {
		return nil, err
	}

	DB = db
	log.Printf("Database connection established with Read-Write splitting and Monitoring.")

	// When the app role is in use, create a separate connection that bypasses
	// RLS for internal telemetry tables (e.g., http_metric_buckets).
	// These tables are written to by a background goroutine and contain only
	// aggregate performance data — no multi-tenant row isolation is needed.
	if useAppRole {
		strippedDSN, err := StripRoleFromDSN(dsn)
		if err != nil {
			log.Printf("[WARN] Failed to strip role from DSN for telemetry connection: %v", err)
		} else {
			rawDB, err := gorm.Open(postgresDialector(strippedDSN), &gorm.Config{
				Logger:         buildGormLogger(),
				PrepareStmt:    !usesPgBouncer(strippedDSN),
				NamingStrategy: LegacySchemaNamingStrategy{},
			})
			if err != nil {
				log.Printf("[WARN] Failed to create raw DB connection for telemetry: %v", err)
			} else {
				rawWriteDB = rawDB
				sqlDB, _ := rawDB.DB()
				sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
				sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
				sqlDB.SetConnMaxLifetime(pool.MaxLifetime)
				sqlDB.SetConnMaxIdleTime(pool.MaxIdleTime)
				log.Printf("[DB] Raw telemetry connection established (bypasses RLS for internal tables)")
			}
		}
	}

	// Verify connection with immediate Ping (fail-fast)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("database ping failed - connection is not functional: %w", err)
	}
	log.Println("Database ready. Schema changes are controlled by explicit migration flags.")

	return db, nil
}

// Close closes the underlying sql.DB connections cleanly.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	log.Println("Closing underlying database connection pool...")
	err1 := sqlDB.Close()

	var err2 error
	if rawWriteDB != nil {
		rawSQL, rawErr := rawWriteDB.DB()
		if rawErr == nil {
			rawSQL.Close()
		}
		err2 = rawErr
	}

	if err1 != nil {
		return err1
	}
	return err2
}

// RawWriteDB returns a GORM session that connects without the app_user role,
// bypassing Row-Level Security. It is intended for internal telemetry tables
// (e.g. http_metric_buckets) that do not require multi-tenant row isolation.
// When the app role is not in use, it falls back to WriteDB().
func RawWriteDB(ctxs ...context.Context) *gorm.DB {
	if rawWriteDB != nil {
		session := rawWriteDB.Session(&gorm.Session{})
		if len(ctxs) > 0 && ctxs[0] != nil {
			session = session.WithContext(ctxs[0])
		}
		return session
	}
	return WriteDB(ctxs...)
}

// RawReadDB returns a GORM session that connects without the app_user role,
// bypassing Row-Level Security for reads on internal telemetry tables.
// Falls back to ReadDB() when the app role is not in use.
func RawReadDB(ctxs ...context.Context) *gorm.DB {
	if rawWriteDB != nil {
		session := rawWriteDB.Session(&gorm.Session{})
		if len(ctxs) > 0 && ctxs[0] != nil {
			session = session.WithContext(ctxs[0])
		}
		return session
	}
	return ReadDB(ctxs...)
}

// ReadDB returns a GORM session explicitly routed to a read replica.
// Use this in all query (read) handlers to enforce CQRS read path.
func ReadDB(ctxs ...context.Context) *gorm.DB {
	if DB == nil {
		return nil
	}
	db := DB.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Read)
	if len(ctxs) > 0 && ctxs[0] != nil {
		db = db.WithContext(ctxs[0])
	}
	return db
}

// WriteDB returns a GORM session explicitly routed to the write source.
// Use this in all command (write) handlers to enforce CQRS write path.
func WriteDB(ctxs ...context.Context) *gorm.DB {
	if DB == nil {
		return nil
	}
	db := DB.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Write)
	if len(ctxs) > 0 && ctxs[0] != nil {
		db = db.WithContext(ctxs[0])
	}
	return db
}

// WarmUpPools pre-opens connections on every CQRS connection pool (read
// replica and write source, plus the raw telemetry pool when present) up to the
// configured idle-connection ceiling.
//
// Background: database/sql opens connections lazily, so right after boot the
// pool is empty. Against a remote database (Supabase/PgBouncer, AWS RDS, ...)
// each NEW connection pays a TCP+TLS+auth handshake of ~300-800ms. Without a
// warm-up, the first few production requests after startup stall behind those
// cold connections — a trivial `SELECT ... LIMIT 1` shows up as a 500ms+
// "slow query" even though the query itself is instant.
//
// This must be called BEFORE the HTTP server starts listening so the first
// requests reuse already-established connections.
func WarmUpPools(ctx context.Context) {
	if DB == nil {
		log.Println("[DB WarmUp] Skipped: database not initialized")
		return
	}

	pool := getPoolSettings()
	target := pool.MaxIdleConns
	if target < 1 {
		target = 1
	}
	if target > pool.MaxOpenConns {
		target = pool.MaxOpenConns
	}
	// Explicit operator override so warm-up depth can be tuned separately from
	// the idle pool settings.
	if v, val := getEnvInt("DB_PREWARM_CONNS"); v && val > 0 {
		target = val
	}
	// Serverless instances share the database — keep the warm-up footprint
	// tiny so N concurrent instances cannot exhaust the PgBouncer pool.
	if isServerlessEnv() && target > 5 {
		target = 5
	}
	// For local development, keep the warm-up footprint minimal.
	// We don't have remote connection handshakes (localhost DB latency is <1ms),
	// so opening many connections is a waste of time and blocks process startup.
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = os.Getenv("GO_ENV")
	}
	if appEnv != "production" && target > 1 {
		// Only override if the user hasn't explicitly set DB_PREWARM_CONNS
		if hasPrewarmOverride, _ := getEnvInt("DB_PREWARM_CONNS"); !hasPrewarmOverride {
			target = 1
		}
	}

	poolGetters := []func(context.Context) *gorm.DB{
		func(c context.Context) *gorm.DB { return ReadDB(c) },
		func(c context.Context) *gorm.DB { return WriteDB(c) },
	}
	if rawWriteDB != nil {
		poolGetters = append(poolGetters, func(c context.Context) *gorm.DB { return RawWriteDB(c) })
	}

	start := time.Now()
	var wg sync.WaitGroup
	for _, get := range poolGetters {
		for i := 0; i < target; i++ {
			wg.Add(1)
			go func(g func(context.Context) *gorm.DB) {
				defer wg.Done()
				qCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				var one int
				if err := g(qCtx).Raw("SELECT 1").Scan(&one).Error; err != nil {
					log.Printf("[DB WarmUp] Warning: failed to establish a pool connection: %v", err)
				}
			}(get)
		}
	}
	wg.Wait()
	log.Printf("[DB WarmUp] Pre-opened %d connection(s) per pool in %s", target, time.Since(start).Round(time.Millisecond))
}

// WithWriteTx executes fn within a write-routed transaction.
// This guarantees all operations in fn go to the write source.
func WithWriteTx(fn func(tx *gorm.DB) error, ctxs ...context.Context) error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	session := DB.Session(&gorm.Session{}).Clauses(dbresolver.Write)
	if len(ctxs) > 0 && ctxs[0] != nil {
		session = session.WithContext(ctxs[0])
	}
	return session.Transaction(fn)
}

func getGormLogLevel() logger.LogLevel {
	// APP_ENV / GO_ENV are the idiomatic Go environment variables.
	// NODE_ENV is a Node.js convention and must not be used here.
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("GO_ENV")
	}
	if (os.Getenv("DB_LOG_LEVEL") == "info" || os.Getenv("DB_DEBUG") == "true") && env != "production" {
		return logger.Info
	}
	return logger.Warn
}

func getReplicaDialectors() []gorm.Dialector {
	replicas := os.Getenv("DATABASE_REPLICAS")
	var replicaDialectors []gorm.Dialector
	if replicas != "" {
		for _, replicaDSN := range strings.Split(replicas, ",") {
			replicaDialectors = append(replicaDialectors, postgresDialector(strings.TrimSpace(replicaDSN)))
		}
	}
	return replicaDialectors
}

func postgresDialector(dsn string) gorm.Dialector {
	if usesPgBouncer(dsn) {
		return postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		})
	}
	return postgres.Open(dsn)
}

func usesPgBouncer(dsn string) bool {
	lowerDSN := strings.ToLower(dsn)
	return strings.Contains(lowerDSN, "pgbouncer=true") ||
		strings.Contains(lowerDSN, ".pooler.supabase.com:6543")
}

type poolSettings struct {
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
	MaxIdleTime  time.Duration
}

// isServerlessEnv detects Vercel, AWS Lambda, and similar ephemeral function environments.
// In these environments each invocation may run in a separate process, meaning every
// instance creates its own connection pool. We must therefore use a very small pool to
// avoid exhausting the database (especially Supabase PgBouncer) under concurrent load.
func isServerlessEnv() bool {
	return os.Getenv("VERCEL") == "1" ||
		os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" ||
		os.Getenv("SERVERLESS") == "1"
}

func getPoolSettings() poolSettings {
	serverless := isServerlessEnv()

	var settings poolSettings
	if serverless {
		// CRITICAL for Vercel/Lambda: each concurrent Lambda instance opens its own pool.
		// With N=1000 concurrent lambdas and MaxOpenConns=300 you would request 300,000
		// connections — destroying the PgBouncer pool instantly.
		// At 5 conns/instance, 1,000 instances = 5,000 total — well within Supabase limits.
		settings = poolSettings{
			MaxIdleConns: 2,
			MaxOpenConns: 5,
			MaxLifetime:  1 * time.Minute,  // Force quick release; Lambda reuse is short-lived
			MaxIdleTime:  30 * time.Second, // Release idle conns before Lambda is frozen
		}
	} else {
		// Traditional always-on server: standard connection pool.
		settings = poolSettings{
			MaxIdleConns: 25,
			MaxOpenConns: 50,
			MaxLifetime:  15 * time.Minute,
			MaxIdleTime:  5 * time.Minute,
		}
	}

	// Allow explicit overrides from environment variables in all cases.
	hasIdle := false
	if v, val := getEnvInt("DB_MAX_IDLE_CONNS"); v {
		settings.MaxIdleConns = val
		hasIdle = true
	}
	if v, val := getEnvInt("DB_MAX_OPEN_CONNS"); v {
		settings.MaxOpenConns = val
		if !hasIdle {
			settings.MaxIdleConns = val / 2
			if settings.MaxIdleConns < 2 {
				settings.MaxIdleConns = 2
			}
		}
	}
	if v, val := getEnvDuration("DB_MAX_LIFETIME"); v {
		settings.MaxLifetime = val
	}
	if v, val := getEnvDuration("DB_MAX_IDLE_TIME"); v {
		settings.MaxIdleTime = val
	}

	if serverless {
		log.Printf("[DB Pool] Serverless environment detected — using connection pool (MaxOpen=%d, MaxIdle=%d)", settings.MaxOpenConns, settings.MaxIdleConns)
	} else {
		log.Printf("[DB Pool] Traditional server environment — using connection pool (MaxOpen=%d, MaxIdle=%d)", settings.MaxOpenConns, settings.MaxIdleConns)
	}

	return settings
}

func getEnvInt(key string) (bool, int) {
	val := os.Getenv(key)
	if val == "" {
		return false, 0
	}
	var intVal int
	_, err := fmt.Sscanf(val, "%d", &intVal)
	if err != nil {
		return false, 0
	}
	return true, intVal
}

func getEnvDuration(key string) (bool, time.Duration) {
	val := os.Getenv(key)
	if val == "" {
		return false, 0
	}
	duration, err := time.ParseDuration(val)
	if err != nil {
		return false, 0
	}
	return true, duration
}
