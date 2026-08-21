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
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"thanawy-backend/internal/application/services"
	systemservice "thanawy-backend/internal/domain/system/service"
	dashboarddelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	contentdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/cache"
	userrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	"time"

	"thanawy-backend/internal/application"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"

	"thanawy-backend/internal/infrastructure/api"

	"thanawy-backend/internal/infrastructure/storage"
	worker "thanawy-backend/internal/infrastructure/workers"
	"thanawy-backend/pkg/buildinfo"
	"thanawy-backend/pkg/telemetry"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"thanawy-backend/internal/infrastructure/api/middleware"

	_ "thanawy-backend/docs" // Required for Swagger documentation generation

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

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
	if err != nil {
		log.Printf("[Telemetry] WARNING: Failed to initialize tracer: %v", err)
	} else {
		defer telemetry.Shutdown(tp)
	}

	// Setup Router
	r := setupRouter(cfg, hexHandlers)

	appMode := strings.ToLower(os.Getenv("APP_MODE"))
	if appMode == "" {
		appMode = strings.ToLower(os.Getenv("MODE"))
	}
	if appMode == "" {
		appMode = "api"
	}

	runAPI := getEnvBool("RUN_API", true)
	runWorkers := getEnvBool("RUN_WORKERS", false)
	runScheduler := getEnvBool("RUN_SCHEDULER", false)

	if appMode != "" {
		switch appMode {
		case "api":
			runAPI = true
			runWorkers = false
			runScheduler = false
		case "worker", "workers":
			runAPI = false
			runWorkers = true
			runScheduler = false
		case "scheduler":
			runAPI = false
			runWorkers = false
			runScheduler = true
		case "all":
			runAPI = true
			runWorkers = true
			runScheduler = true
		}
	}

	log.Printf("[Process Mode] APP_MODE=%s (runAPI=%t, runWorkers=%t, runScheduler=%t)",
		appMode, runAPI, runWorkers, runScheduler)

	var srv *http.Server

	// workerCtx is cancelled on shutdown signal to stop all worker goroutines.
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	if runWorkers {
		// Start Database-backed Mail Queue Worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("Starting background mail queue worker...")
			services.GetMailQueueWorker().Start()
		}()

		// Start Automated Database Backup Scheduler
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("Starting automated database backup scheduler...")
			systemservice.GetBackupCronWorker().Start()
		}()

		if cache.Redis != nil {
			// Start Asynq Background Worker (shuts down when workerCtx is cancelled)
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Println("Starting background worker...")
				worker.StartWorkerWithContext(workerCtx)
			}()

			// Start Analytics Batch Worker (separate from Asynq — uses Redis Stream)
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Println("Starting analytics batch worker (Redis Stream consumer)...")
				worker.StartAnalyticsBatchWorkerWithContext(workerCtx)
			}()
		} else {
			log.Println("Redis is not connected; background Redis workers are disabled for this process")
		}
	}

	if runScheduler && cache.Redis != nil {
		go func() {
			log.Println("Starting periodic task scheduler...")
			worker.StartScheduler()
		}()
	}
	if runScheduler && cache.Redis == nil {
		log.Println("Redis is not connected; periodic scheduler is disabled for this process")
	}

	if runAPI {
		// Start HTTP Server with graceful shutdown
		port := os.Getenv("BACKEND_PORT")
		if port == "" {
			port = os.Getenv("PORT")
		}

		if port == "" {
			port = "8082"
		}

		readTimeout, err := time.ParseDuration(cfg.HTTPReadTimeout)
		if err != nil {
			readTimeout = 10 * time.Second
		}
		writeTimeout, err := time.ParseDuration(cfg.HTTPWriteTimeout)
		if err != nil {
			writeTimeout = 30 * time.Second
		}
		idleTimeout, err := time.ParseDuration(cfg.HTTPIdleTimeout)
		if err != nil {
			idleTimeout = 120 * time.Second
		}

		srv = &http.Server{
			Addr:         ":" + port,
			Handler:      r,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		}

		// Run server in goroutine
		go func() {
			log.Printf("HTTP server starting on port %s", port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start server: %v", err)
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown processes
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down processes...")

	// 1. Stop all background workers first (before closing DB/Redis)
	log.Println("Stopping background workers...")
	stopWorkers()

	if runWorkers {
		services.GetMailQueueWorker().Stop()
		systemservice.GetBackupCronWorker().Stop()
	}

	// 2. Wait for all worker goroutines to exit (max 30 seconds)
	workerDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workerDone)
	}()
	select {
	case <-workerDone:
		log.Println("All background workers stopped cleanly")
	case <-time.After(30 * time.Second):
		log.Println("[WARN] Timed out waiting for workers to stop; proceeding with shutdown")
	}

	// 3. Shutdown HTTP server after workers are done
	if runAPI && srv != nil {
		log.Println("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server forced to shutdown: %v", err)
		}
	}

	// 4. Close Redis and DB connections last
	if err := cache.CloseRedis(); err != nil {
		log.Printf("Error closing Redis connection pool: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("Error closing database connection pool: %v", err)
	}

	log.Println("Process exited")
}

func connectDatabaseWithRetry(cfg *config.Config) (*gorm.DB, error) {
	attempts := getEnvInt("DB_CONNECT_ATTEMPTS", 5)
	delay := time.Duration(getEnvInt("DB_CONNECT_RETRY_SECONDS", 2)) * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		database, err := db.ConnectWithWriteDSN(cfg.DatabaseURL, cfg.DatabaseWriteURL)
		if err == nil {
			if attempt > 1 {
				log.Printf("Database connected after %d attempts", attempt)
			}
			return database, nil
		}

		lastErr = err
		if attempt == attempts {
			break
		}

		log.Printf("Database is not ready yet (attempt %d/%d): %v. Retrying in %s...", attempt, attempts, err, delay)
		time.Sleep(delay)
	}

	return nil, lastErr
}

func initStorage(cfg *config.Config) {
	if cfg.StorageType == "local" {
		baseDir := os.Getenv("STORAGE_PATH")
		if baseDir == "" {
			baseDir = "./uploads"
		}
		publicURL := os.Getenv("STORAGE_PUBLIC_URL")
		storageSvc, err := storage.NewLocalStorage(baseDir, publicURL)
		if err != nil {
			log.Fatalf("Failed to initialize local storage: %v", err)
		}
		storage.GlobalStorage = storageSvc
		log.Printf("Storage initialized with local provider at %s", baseDir)
		return
	}

	if cfg.StorageType != "s3" {
		log.Fatalf("Unsupported storage provider %q. Cloud storage (s3) is required.", cfg.StorageType)
	}
	if cfg.S3.Endpoint == "" {
		log.Fatal("S3_ENDPOINT is required for cloud storage")
	}

	storageSvc, err := storage.NewS3Storage(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.Bucket,
		cfg.S3.Region,
		cfg.S3.UseSSL,
		cfg.S3.PublicURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize S3 storage: %v", err)
	}
	storage.GlobalStorage = storageSvc
	log.Println("Storage initialized with S3 provider (Cloudflare R2)")
}

func setupRouter(cfg *config.Config, hexHandlers *application.Handlers) *gin.Engine {
	if os.Getenv("GIN_MODE") == "release" || cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	if cfg.SentryDSN != "" {
		r.Use(sentrygin.New(sentrygin.Options{
			Repanic: true,
		}))
	}

	if cfg.TrustProxy {
		if len(cfg.TrustedProxies) > 0 {
			if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
				log.Printf("WARNING: Failed to set trusted proxies: %v", err)
			} else {
				log.Printf("Trusted proxies configured: %v", cfg.TrustedProxies)
			}
		} else {
			// Trust all proxies (Gin default) but set explicitly to suppress startup warning
			_ = r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
			log.Println("Trusting all proxies (0.0.0.0/0, ::/0)")
		}
	} else {
		_ = r.SetTrustedProxies(nil)
		log.Println("Proxy trusting disabled (SetTrustedProxies(nil))")
	}

	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.ValidateSecrets(middleware.DefaultSecretsValidatorConfig()))
	r.Use(middleware.PerformanceMonitor())
	r.Use(telemetry.TraceMiddleware())

	rateLimitWindow, err := time.ParseDuration(cfg.RateLimitWindow)
	if err != nil {
		rateLimitWindow = time.Minute
	}
	r.Use(middleware.GlobalRateLimiter(cfg.RateLimitRequests, rateLimitWindow))

	r.Use(middleware.CSRFMiddleware())
	r.Use(middleware.DBConsistencyMiddleware(db.DB))

	// Serve uploaded media. With local storage the file key (including its
	// folder segment, e.g. "uploads/<uuid>.png") maps 1:1 to the path under
	// STORAGE_PATH, so the files are served straight from disk. With cloud
	// storage, media URLs are absolute CDN/S3 URLs, so local serving is
	// disabled and any stray local path request is answered with 410.
	if cfg.StorageType == "local" {
		localBaseDir := os.Getenv("STORAGE_PATH")
		if localBaseDir == "" {
			localBaseDir = "./uploads"
		}
		r.Static("/uploads", localBaseDir)
	} else {
		r.GET("/uploads/*path", func(c *gin.Context) {
			c.JSON(http.StatusGone, gin.H{
				"error": "Local file serving is disabled. All media is served from cloud storage.",
			})
		})
	}

	rootHandler := func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "UP",
			"message":   "Thanawy Backend API is running",
			"version":   buildinfo.Version,
			"commit":    buildinfo.Commit,
			"buildTime": buildinfo.BuildTime,
		})
	}
	r.GET("/", rootHandler)

	// Public health check routes (bypass configuration validation and rate limits)
	r.GET("/health", func(c *gin.Context) {
		redisOK := true
		redisLatencyMs := int64(-1)
		if cache.IsRedisAvailable() {
			start := time.Now()
			if err := cache.RedisHealthCheck(c.Request.Context()); err != nil {
				redisOK = false
			} else {
				redisLatencyMs = time.Since(start).Milliseconds()
			}
		} else {
			// Redis is intentionally disabled — not an error.
			redisLatencyMs = 0
		}
		c.JSON(200, gin.H{
			"status":           "UP",
			"redis_ok":         redisOK,
			"redis_latency_ms": redisLatencyMs,
		})
	})
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/readyz", func(c *gin.Context) {
		// readyz signals that the process is ready to serve traffic.
		// Fail if Redis is configured but not reachable.
		if cache.IsRedisAvailable() {
			if err := cache.RedisHealthCheck(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "not ready",
					"reason": "redis ping failed: " + err.Error(),
				})
				return
			}
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api.SetupPublicRoutes(r)
	api.SetupProtectedRoutes(r)
	api.SetupAdminRoutes(r)

	// Hexagonal Architecture routes (new)
	api.SetupHexagonalRoutes(r, hexHandlers)

	return r
}

func getEnvBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}
