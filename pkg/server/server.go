// Package server provides a public wrapper for the Vercel serverless function entry point.
// This package can safely import internal packages and is used by the thin api/index.go handler.
package server

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"thanawy-backend/internal/app"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/repository"
	"thanawy-backend/internal/router"
	"thanawy-backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	internalgrpc "thanawy-backend/internal/api/grpc"
	"thanawy-backend/internal/api/handlers"
	"thanawy-backend/internal/middleware"
	thanawyv1connect "thanawy-backend/internal/proto/thanawy/v1/thanawyv1connect"

	_ "thanawy-backend/docs" // Required for Swagger documentation generation

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	engine *gin.Engine
	once   sync.Once
)

func initApp() {
	once.Do(func() {
		// Load environment variables
		_ = godotenv.Load(".env.local")
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, using system environment variables")
		}

		// Initialize Configuration
		cfg := config.Load()
		config.GlobalConfig = cfg

		// Initialize Database with explicit Read/Write DSNs for CQRS
		_, err := db.ConnectWithWriteDSN(cfg.DatabaseURL, cfg.DatabaseWriteURL)
		if err != nil {
			// CRITICAL: Do not silently start without a DB — every handler would panic.
			// Instead, expose a degraded router that returns 503 with a clear message.
			// Vercel will retry on the next invocation; once DB recovers, a fresh Lambda
			// cold start will succeed.
			log.Printf("[CRITICAL] Database connection failed: %v", err)
			log.Println("[CRITICAL] Starting in DEGRADED mode — all /api/* routes return 503")
			engine = buildDegradedRouter(err)
			return
		}

		// Initialize AuthService with UserRepository dependency
		handlers.InitAuthService(repository.NewUserRepository(db.DB))

		// Initialize Storage (S3 or Local)
		if cfg.StorageType == "local" {
			storageSvc, err := storage.NewLocalStorage(cfg.LocalStorage.BaseDir, cfg.LocalStorage.PublicURL)
			if err != nil {
				log.Printf("Failed to initialize local storage: %v", err)
			} else {
				storage.GlobalStorage = storageSvc
				log.Printf("Storage initialized with Local provider (baseDir: %s, publicURL: %s)", cfg.LocalStorage.BaseDir, cfg.LocalStorage.PublicURL)
			}
		} else if cfg.StorageType == "s3" {
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
				log.Printf("Failed to initialize S3 storage: %v", err)
			} else {
				storage.GlobalStorage = storageSvc
				log.Println("Storage initialized with S3 provider (Cloudflare R2)")
			}
		}

		// Initialize Redis
		redisURL := os.Getenv("REDIS_URL")
		if redisURL != "" {
			db.ConnectRedis(redisURL)
		}

		// Initialize Hexagonal Architecture (Dependency Injection)
		_, hexHandlers := app.Initialize(db.DB)

		// Initialize WebSocket Hub with Redis Pub/Sub support
		handlers.InitHub()

		// Initialize Services for gRPC/Connect
		courseSvc := &internalgrpc.CourseServiceServer{}
		authSvc := internalgrpc.NewAuthServiceServer()
		analyticsSvc := &internalgrpc.AnalyticsServiceServer{}

		// Setup Router
		engine = setupRouter(cfg, hexHandlers, courseSvc, authSvc, analyticsSvc)
	})
}

// buildDegradedRouter returns a minimal Gin engine that serves only the health
// check endpoint and returns 503 Service Unavailable for everything else.
// This is used when the database connection fails at startup so that:
//  1. /health returns 200 (telling Vercel the process is alive)
//  2. /api/healthz returns 503 (signalling degraded state to monitoring)
//  3. All other routes return 503 with a clear JSON body
func buildDegradedRouter(dbErr error) *gin.Engine {
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	errMsg := "Service temporarily unavailable"
	if dbErr != nil {
		errMsg = dbErr.Error()
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP", "database": "degraded"})
	})
	r.GET("/api/healthz", func(c *gin.Context) {
		c.JSON(503, gin.H{"status": "degraded", "error": errMsg})
	})
	r.NoRoute(func(c *gin.Context) {
		c.JSON(503, gin.H{
			"error":   "Service temporarily unavailable — database connection failed",
			"hint":    "The backend failed to connect to the database. Check DATABASE_URL and Supabase connection limits.",
			"details": errMsg,
		})
	})
	return r
}

// Handler is the entrypoint for Vercel Serverless Functions
func Handler(w http.ResponseWriter, r *http.Request) {
	initApp()
	engine.ServeHTTP(w, r)
}

func setupRouter(cfg *config.Config, hexHandlers *app.Handlers, courseSvc *internalgrpc.CourseServiceServer, authSvc *internalgrpc.AuthServiceServer, analyticsSvc *internalgrpc.AnalyticsServiceServer) *gin.Engine {
	if os.Getenv("GIN_MODE") == "release" || cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// Login redirect to /api/auth/login (for backward compatibility)
	r.GET("/login", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/auth/login")
	})

	// Public health check routes (bypass configuration validation and rate limits)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})
	r.GET("/api/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/readyz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ready"})
	})

	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.ValidateSecrets(middleware.DefaultSecretsValidatorConfig()))
	r.Use(middleware.PerformanceMonitor())
	r.Use(middleware.GlobalRateLimiter(200, time.Minute))
	r.Use(middleware.CSRFMiddleware())
	r.Use(middleware.DBConsistencyMiddleware(db.DB))

	// Serve uploaded files statically — LOCAL DEV ONLY.
	// In production (S3/Supabase), all media is served from the CDN.
	// The /uploads route is disabled in cloud deployments to enforce stateless architecture.
	if cfg.StorageType == "local" {
		r.Static("/uploads", cfg.LocalStorage.BaseDir)

		// Support raw PUT uploads for local dev simulation of direct S3 upload
		r.PUT("/uploads/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			cleaned := filepath.Clean(filename)
			if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
				return
			}
			targetPath := filepath.Join(cfg.LocalStorage.BaseDir, cleaned)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			dst, err := os.Create(targetPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer dst.Close()
			_, err = io.Copy(dst, c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "uploaded"})
		})
	} else {
		// Cloud storage active: /uploads/* is not served from this server.
		// Return 410 Gone to surface misconfigurations early.
		r.GET("/uploads/*path", func(c *gin.Context) {
			c.JSON(http.StatusGone, gin.H{
				"error": "Local file serving is disabled. All media is served from cloud storage.",
			})
		})
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Register Connect-RPC Handlers
	coursePath, courseHandler := thanawyv1connect.NewCourseServiceHandler(&internalgrpc.CourseConnectHandler{Svc: courseSvc})
	authPath, authHandler := thanawyv1connect.NewAuthServiceHandler(&internalgrpc.AuthConnectHandler{Svc: authSvc})
	analyticsPath, analyticsHandler := thanawyv1connect.NewAnalyticsServiceHandler(&internalgrpc.AnalyticsConnectHandler{Svc: analyticsSvc})

	r.Any(coursePath+"*any", middleware.OptionalAuth(), gin.WrapH(courseHandler))
	r.Any(authPath+"*any", middleware.OptionalAuth(), gin.WrapH(authHandler))
	r.Any(analyticsPath+"*any", middleware.OptionalAuth(), gin.WrapH(analyticsHandler))

	// Register /api prefixed Connect-RPC Handlers for Vercel routing support
	r.Any("/api"+coursePath+"*any", middleware.OptionalAuth(), gin.WrapH(stripAPIPrefix(courseHandler)))
	r.Any("/api"+authPath+"*any", middleware.OptionalAuth(), gin.WrapH(stripAPIPrefix(authHandler)))
	r.Any("/api"+analyticsPath+"*any", middleware.OptionalAuth(), gin.WrapH(stripAPIPrefix(analyticsHandler)))

	router.SetupAuthRoutes(r)
	router.SetupPublicRoutes(r)
	router.SetupProtectedRoutes(r)
	router.SetupAdminRoutes(r)

	// Hexagonal Architecture routes
	router.SetupHexagonalRoutes(r, hexHandlers)

	return r
}

func stripAPIPrefix(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			if r2.RequestURI != "" {
				r2.RequestURI = strings.TrimPrefix(r2.RequestURI, "/api")
			}
			h.ServeHTTP(w, r2)
			return
		}
		h.ServeHTTP(w, r)
	})
}