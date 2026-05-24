// Package server provides a public wrapper for the Vercel serverless function entry point.
// This package can safely import internal packages and is used by the thin api/index.go handler.
package server

import (
	"log"
	"net/http"
	"os"
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
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, using system environment variables")
		}

		// Initialize Configuration
		cfg := config.Load()
		config.GlobalConfig = cfg

		// Initialize Database with explicit Read/Write DSNs for CQRS
		_, err := db.ConnectWithWriteDSN(cfg.DatabaseURL, cfg.DatabaseWriteURL)
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
		}

		// Initialize AuthService with UserRepository dependency
		handlers.InitAuthService(repository.NewUserRepository(db.DB))

		// Initialize S3 Storage (Cloudflare R2 / AWS S3 / MinIO)
		if cfg.StorageType == "s3" {
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

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Register Connect-RPC Handlers
	coursePath, courseHandler := thanawyv1connect.NewCourseServiceHandler(&internalgrpc.CourseConnectHandler{Svc: courseSvc})
	authPath, authHandler := thanawyv1connect.NewAuthServiceHandler(&internalgrpc.AuthConnectHandler{Svc: authSvc})
	analyticsPath, analyticsHandler := thanawyv1connect.NewAnalyticsServiceHandler(&internalgrpc.AnalyticsConnectHandler{Svc: analyticsSvc})

	r.Any(coursePath+"*any", gin.WrapH(courseHandler))
	r.Any(authPath+"*any", gin.WrapH(authHandler))
	r.Any(analyticsPath+"*any", gin.WrapH(analyticsHandler))

	router.SetupAuthRoutes(r)
	router.SetupPublicRoutes(r)
	router.SetupProtectedRoutes(r)
	router.SetupAdminRoutes(r)

	// Hexagonal Architecture routes
	router.SetupHexagonalRoutes(r, hexHandlers)

	return r
}