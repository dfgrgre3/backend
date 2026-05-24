package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"thanawy-backend/internal/app"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/repository"
	"thanawy-backend/internal/router"
	"thanawy-backend/internal/storage"
	"thanawy-backend/internal/worker"

	internalgrpc "thanawy-backend/internal/api/grpc"
	"thanawy-backend/internal/api/handlers"
	"thanawy-backend/internal/proto/thanawy/v1/thanawyv1connect"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "thanawy-backend/docs"
)

var (
	appHandler http.Handler
	initOnce   sync.Once
	initError  error
)

// Handler is the entry point for Vercel serverless function.
// Vercel's Go runtime detects this exported function.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		appHandler, initError = initializeApp()
		if initError != nil {
			log.Printf("FATAL: Application initialization failed: %v", initError)
		}
	})

	if initError != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("server initialization failed: %v", initError),
		})
		return
	}

	appHandler.ServeHTTP(w, r)
}

// main is required for `go build` of a main package.
// On Vercel, main() exits quickly and the Handler function handles all requests.
// For local development, main() starts the HTTP server normally.
func main() {
	// For Vercel: if the VERCEL environment variable is set, exit quickly.
	// The Handler function will be used instead for each request.
	if os.Getenv("VERCEL") != "" {
		log.Println("Running on Vercel serverless - Handler will be used for each request")
		return
	}

	// Local development: initialize and start HTTP server
	handler, err := initializeApp()
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("HTTP server starting on port %s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initializeApp() (http.Handler, error) {
	// Load configuration safely (no log.Fatal)
	cfg, err := config.LoadSafe()
	if err != nil {
		return nil, fmt.Errorf("config load failed: %w", err)
	}
	config.GlobalConfig = cfg

	// Connect to database
	database, err := db.ConnectWithWriteDSN(cfg.DatabaseURL, cfg.DatabaseWriteURL)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	handlers.InitAuthService(repository.NewUserRepository(database))

	// Initialize S3 Storage (non-fatal on failure)
	initS3Storage(cfg)

	// Initialize Redis (optional)
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		db.ConnectRedis(redisURL)
	}

	// Initialize Hexagonal Architecture
	_, hexHandlers := app.Initialize(database)

	// Initialize WebSocket Hub
	handlers.InitHub()

	// Initialize gRPC services
	courseSvc := &internalgrpc.CourseServiceServer{}
	authSvc := internalgrpc.NewAuthServiceServer()
	analyticsSvc := &internalgrpc.AnalyticsServiceServer{}

	// Setup router
	r := setupRouter(cfg, hexHandlers, courseSvc, authSvc, analyticsSvc)

	// Start workers in background (non-blocking for serverless)
	// Do not run background workers or scheduler on Vercel serverless environment
	if os.Getenv("VERCEL") == "" {
		if os.Getenv("RUN_WORKERS") != "false" {
			go func() {
				worker.StartWorker()
			}()
			go func() {
				worker.StartAnalyticsBatchWorker()
			}()
		}

		if os.Getenv("RUN_SCHEDULER") != "false" {
			go func() {
				worker.StartScheduler()
			}()
		}
	}

	return r, nil
}

func initS3Storage(cfg *config.Config) {
	if cfg.StorageType != "s3" || cfg.S3.Endpoint == "" {
		log.Println("S3 storage not configured or endpoint is empty, skipping initialization")
		return
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
		log.Printf("Failed to initialize S3 storage: %v", err)
		return
	}
	storage.GlobalStorage = storageSvc
	log.Println("Storage initialized with S3 provider (Cloudflare R2)")
}

func setupRouter(cfg *config.Config, hexHandlers *app.Handlers, courseSvc *internalgrpc.CourseServiceServer, authSvc *internalgrpc.AuthServiceServer, analyticsSvc *internalgrpc.AnalyticsServiceServer) *gin.Engine {
	if os.Getenv("GIN_MODE") == "release" || cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// Vercel rewrites all paths to /api, so we must respond for both / and /api.
	rootHandler := func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "UP",
			"message": "Thanawy Backend API is running",
			"version": "1.0",
		})
	}
	r.GET("/", rootHandler)
	r.GET("/api", rootHandler)

	// Public health check routes (bypass configuration validation and rate limits)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})
	r.GET("/api/health", func(c *gin.Context) {
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

	// Hexagonal Architecture routes (new)
	router.SetupHexagonalRoutes(r, hexHandlers)

	return r
}