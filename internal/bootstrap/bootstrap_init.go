package bootstrap

import (
	"context"
	"log"
	"os"
	"thanawy-backend/internal/application"
	dashboarddelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	contentdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	userrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	"thanawy-backend/pkg/telemetry"
	"time"
)

// initInfrastructure connects the database, warms up connection pools and
// repository schema caches, initializes storage, connects Redis, wires the
// hexagonal-architecture dependency graph, starts the WebSocket hub and
// initializes the OpenTelemetry tracer.
//
// It returns the hexagonal handlers used by setupRouter, and a cleanup
// function the caller must defer (mirrors the original inline
// `defer telemetry.Shutdown(tp)` in Run() — that defer has to fire when the
// whole process is shutting down, not when this initialization step
// returns, so the deferred call itself must live in Run(), not here).
func (a *app) initInfrastructure() (*application.Handlers, func()) {
	cfg := a.cfg

	// Initialize Database with explicit Read/Write DSNs for CQRS
	_, err := connectDatabaseWithRetry(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if len(cfg.DatabaseReadReplicas) > 0 {
		log.Printf("Database configured with %d read replica(s)", len(cfg.DatabaseReadReplicas))
	}

	// Pre-open connections on every DB pool (read replica + write source +
	// telemetry) BEFORE the HTTP server starts listening. database/sql opens
	// connections lazily, so without this the first requests after boot pay a
	// 300-800ms cold TCP+TLS+auth handshake each — observed as 500ms+ "slow
	// queries" on trivial SELECTs and multi-second first-page loads.
	db.WarmUpPools(context.Background())

	// Warmup repository schema checks to avoid slow INFORMATION_SCHEMA queries
	// on the first request (e.g. GORM Migrator.HasColumn for deleted_at).
	userrepo.WarmupUserRepository()
	contentdelivery.WarmupBookRepository()
	dashboarddelivery.WarmupUserSubscriptionRepository()

	// Initialize Storage (Local or S3)
	initStorage(cfg)

	// SQL migrations and seeding are now handled by a separate process (cmd/migrate/main.go)
	// to avoid race conditions in distributed environments.

	// Initialize Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		redisCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		if err := cache.ConnectRedis(redisCtx, redisURL); err != nil {
			log.Printf("Redis unavailable; Redis-backed capabilities will remain disabled: %v", err)
		}
		cancel()
	}

	// Initialize Hexagonal Architecture (Dependency Injection)
	hexHandlers, err := application.Initialize(db.DB)
	if err != nil {
		log.Printf("Warning: Failed to initialize hexagonal handlers: %v", err)
	}

	// Initialize WebSocket Hub with Redis Pub/Sub support
	contentdelivery.InitHub()

	// Initialize OpenTelemetry tracer
	tp, err := telemetry.InitTracer("thanawy-backend")
	cleanup := func() {}
	if err != nil {
		log.Printf("[Telemetry] WARNING: Failed to initialize tracer: %v", err)
	} else {
		cleanup = func() { telemetry.Shutdown(tp) }
	}

	return hexHandlers, cleanup
}
