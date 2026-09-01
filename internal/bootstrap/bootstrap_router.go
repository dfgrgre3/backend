package bootstrap

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"thanawy-backend/internal/application"
	"thanawy-backend/internal/infrastructure/api"
	"thanawy-backend/internal/infrastructure/api/middleware"
	"thanawy-backend/internal/infrastructure/cache"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/monitoring"
	"thanawy-backend/pkg/buildinfo"
	"thanawy-backend/pkg/telemetry"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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

	r.Use(redactingLogger())
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS())

	// Prometheus metrics: the collector/middleware/label definitions already
	// existed (internal/infrastructure/monitoring/profiler.go) but were never
	// instantiated or wired into the router, so the client_golang dependency
	// was dead weight and no scrape target existed. This is a separate,
	// scrape-only endpoint from the existing custom JSON /api/metrics.
	metricsCollector := monitoring.NewMetricsCollector()
	r.Use(monitoring.ProfilingMiddleware(metricsCollector))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
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

// sensitiveQueryParams are query-string keys whose values must never be
// written to the access log. The WebSocket handshake authenticates with the
// access token in the query string (the browser WebSocket API cannot send
// custom headers), so without redaction every reconnect leaks a fresh JWT
// into the logs.
var sensitiveQueryParams = map[string]bool{
	"access_token": true,
	"token":        true,
	"api_key":      true,
	"apikey":       true,
	"code":         true,
	"state":        true,
}

// redactQuery masks sensitive values in a raw query string while keeping the
// key present (so logs still show whether auth params were supplied).
func redactQuery(rawQuery string) string {
	if rawQuery == "" {
		return rawQuery
	}
	q := make(url.Values)
	parsed, _ := url.ParseQuery(rawQuery)
	for key, values := range parsed {
		if sensitiveQueryParams[key] {
			for range values {
				q.Add(key, "[REDACTED]")
			}
			continue
		}
		for _, v := range values {
			q.Add(key, v)
		}
	}
	return q.Encode()
}

// redactingLogger mirrors gin.Logger() but masks sensitive query parameters
// before writing the request line to the log.
func redactingLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		path := param.Request.URL.Path
		if raw := param.Request.URL.RawQuery; raw != "" {
			path += "?" + redactQuery(raw)
		}
		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			path,
			param.ErrorMessage,
		)
	})
}
