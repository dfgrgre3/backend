package db

import (
	"context"
	"fmt"
	"log"
	"os"
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
		sourceDSN = writeDSN
		if useAppRole {
			// For write DSN, also apply app role to respect RLS.
			writeAppDSN, err := GetDSNForRole(writeDSN, RoleApp)
			if err != nil {
				log.Printf("[WARN] Failed to add app role to write DSN: %v", err)
			} else {
				sourceDSN = writeAppDSN
			}
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
