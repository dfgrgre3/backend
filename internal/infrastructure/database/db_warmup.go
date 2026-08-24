package db

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/gorm"
)

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
