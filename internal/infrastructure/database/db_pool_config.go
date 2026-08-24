package db

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
