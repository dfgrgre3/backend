package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var DB *gorm.DB

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
		log.Printf("[DB] Using direct database connection (app role disabled)")
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
	log.Println("Database ready. Schema changes are controlled by explicit migration flags.")

	return db, nil
}

// ConnectForMigration creates a connection with migration_user role for schema changes.
// NOTE: this does NOT set the global DB variable — migration connections are
// intentionally kept separate from the application connection pool.
func ConnectForMigration(dsn string) (*gorm.DB, error) {
	// Use migration role for schema changes.
	migrationDSN, err := GetMigrationDSN()
	if err != nil {
		log.Printf("[WARN] Failed to get migration DSN, falling back to provided DSN: %v", err)
		migrationDSN = dsn
	} else {
		log.Printf("[DB] Using migration_user role for schema changes")
	}

	db, err := gorm.Open(postgres.Open(migrationDSN), &gorm.Config{
		Logger:         buildGormLogger(),
		PrepareStmt:    !usesPgBouncer(migrationDSN),
		NamingStrategy: LegacySchemaNamingStrategy{},
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	pool := getPoolSettings()
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(pool.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(pool.MaxIdleTime)

	// Do NOT assign to DB — migration connections are independent of the
	// application read/write pool to avoid corrupting the CQRS routing.
	log.Printf("Database connection established for migrations with migration_user role")

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
	return sqlDB.Close()
}

// ReadDB returns a GORM session explicitly routed to a read replica.
// Use this in all query (read) handlers to enforce CQRS read path.
func ReadDB(ctxs ...context.Context) *gorm.DB {
	if DB == nil {
		return nil
	}
	db := DB.Session(&gorm.Session{}).Clauses(dbresolver.Read)
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
	db := DB.Session(&gorm.Session{}).Clauses(dbresolver.Write)
	if len(ctxs) > 0 && ctxs[0] != nil {
		db = db.WithContext(ctxs[0])
	}
	return db
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
