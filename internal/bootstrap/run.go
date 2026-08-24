package bootstrap

// @title Thanawy API
// @version 1.0
// @description This is the API server for the Thanawy platform.
// @host localhost:8082
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"thanawy-backend/internal/infrastructure/config"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"

	_ "thanawy-backend/docs" // Required for Swagger documentation generation
)

// app carries the shared state threaded through Run()'s startup, run and
// shutdown phases (previously local variables in one long function). Its
// methods are split across several files in this package (all sharing
// package bootstrap): this file (Run orchestrator), bootstrap_app.go (the
// app struct + mode resolution), bootstrap_init.go (early
// DB/storage/redis/telemetry/DI initialization), bootstrap_workers.go
// (background worker/scheduler startup), bootstrap_server.go (HTTP server
// startup), bootstrap_shutdown.go (graceful shutdown), bootstrap_db.go
// (connectDatabaseWithRetry), bootstrap_storage.go (initStorage),
// bootstrap_router.go (setupRouter) and bootstrap_env.go (env var helpers).
type app struct {
	cfg *config.Config

	runAPI       bool
	runWorkers   bool
	runScheduler bool

	wg          sync.WaitGroup
	workerCtx   context.Context
	stopWorkers context.CancelFunc

	srv *http.Server
}

func Run() {
	// Log startup in UTC to avoid timezone confusion in logs
	log.SetFlags(0) // We control the format below
	log.Println(strings.Repeat("=", 60))
	log.Printf("[STARTUP] Thanawy Backend starting at %s (UTC)", time.Now().UTC().Format(time.RFC3339))

	// Log configuration source
	if os.Getenv("APP_ENV") == "" || os.Getenv("APP_ENV") == "development" {
		if _, err := os.Stat(".env"); err == nil {
			log.Println("[CONFIG] Loading configuration from .env file")
		} else {
			log.Println("[CONFIG] Using system environment variables (no .env file found)")
		}
	} else {
		log.Println("[CONFIG] Using environment variables (production mode)")
	}

	// Load environment variables only in development
	if os.Getenv("APP_ENV") == "" || os.Getenv("APP_ENV") == "development" {
		if err := godotenv.Load(); err != nil {
			log.Printf("Failed to load .env: %v, using system environment variables", err)
		}
	}

	// Initialize Configuration
	cfg := config.Load()
	config.GlobalConfig = cfg

	// Initialize Sentry SDK early
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			Environment:      cfg.AppEnv,
			Release:          cfg.AppVersion,
			AttachStacktrace: true,
		}); err != nil {
			log.Printf("Sentry initialization failed: %v\n", err)
		} else {
			log.Println("Sentry SDK initialized successfully")
			// Flush buffered events before the program terminates
			defer sentry.Flush(2 * time.Second)
		}
	}

	a := &app{cfg: cfg}

	hexHandlers, telemetryCleanup := a.initInfrastructure()
	defer telemetryCleanup()

	// Setup Router
	r := setupRouter(cfg, hexHandlers)

	a.resolveMode()

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	a.workerCtx = workerCtx
	a.stopWorkers = stopWorkers

	a.startWorkers()
	a.startScheduler()
	a.startAPIServer(r)

	a.waitForShutdownSignal()
	a.shutdown()
}
