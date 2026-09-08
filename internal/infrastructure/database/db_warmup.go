package db

import (
	"context"
	"log"
	"net/url"
	"os"
	"strings"
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
	// For local development against a LOCAL database, keep the warm-up
	// footprint minimal: the localhost handshake is <1ms, so pre-opening many
	// connections only wastes time and blocks process startup.
	// When the dev machine talks to a REMOTE database (cloud Postgres, SSH
	// tunnel, ...), every lazily-opened connection pays a full TCP+TLS+auth
	// handshake (~300-1000ms) that surfaces as bogus SLOW SQL entries on real
	// queries — keep the full warm-up depth in that case.
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = os.Getenv("GO_ENV")
	}
	if appEnv != "production" && target > 1 && isLocalDatabase() {
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

// isLocalDatabase reports whether every active connection pool points at a
// database on this machine (loopback or the Docker host). Against such a
// database the connection handshake is effectively free, so a deep warm-up is
// pointless. Any remote host (cloud Postgres, Supabase, ...) requires the
// full warm-up because each cold connection stalls the first query behind a
// TCP+TLS+auth handshake.
func isLocalDatabase() bool {
	if len(poolDSNs) == 0 {
		return false // Unknown — assume remote so warm-up stays safe.
	}
	for _, dsn := range poolDSNs {
		if !isLocalHost(dsnHost(dsn)) {
			return false
		}
	}
	return true
}

// isLocalHost reports whether a DSN host refers to this machine. An empty
// host means a Unix-domain socket (libpq default), which is always local.
func isLocalHost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "", "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	}
	return false
}

// dsnHost extracts the host from a Postgres DSN in either URL form
// (postgres://user:pass@host:5432/db) or keyword/value form (host=x ...).
func dsnHost(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if u, err := url.Parse(dsn); err == nil {
			return u.Hostname()
		}
		return ""
	}
	for _, field := range strings.Fields(dsn) {
		if key, value, ok := strings.Cut(field, "="); ok && strings.EqualFold(key, "host") {
			return strings.Trim(value, `'"`)
		}
	}
	return ""
}
