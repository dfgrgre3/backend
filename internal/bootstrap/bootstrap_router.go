package bootstrap

import (
	"log"
	"net/http"
	"os"
	"time"

	"thanawy-backend/internal/application"
	"thanawy-backend/internal/infrastructure/api"
	"thanawy-backend/internal/infrastructure/api/middleware"
	"thanawy-backend/internal/infrastructure/cache"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/pkg/buildinfo"
	"thanawy-backend/pkg/telemetry"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

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
	api.SetupProtectedRoutes(r, hexHandlers)
	api.SetupAdminRoutes(r)

	// Hexagonal Architecture routes (new)
	api.SetupHexagonalRoutes(r, hexHandlers)

	return r
}
