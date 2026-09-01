package worker

import (
	"context"
	"log"
	"os"
	"strconv"
	"thanawy-backend/internal/infrastructure/cache"

	"github.com/hibiken/asynq"
)

// getAsynqConcurrency returns the concurrency setting from env var or default
func getAsynqConcurrency() int {
	if val := os.Getenv("ASYNQ_CONCURRENCY"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 10 // Default concurrency
}

// StartWorkerWithContext starts the Asynq worker server. It blocks until ctx
// is cancelled, at which point it performs a graceful shutdown of the server.
func StartWorkerWithContext(ctx context.Context) {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" || isRedisDisabled() {
		log.Println("[Worker] Redis not configured or disabled, skipping worker start")
		return
	}

	opts := cache.ParseAsynqRedisConnOpt(redisAddr)

	concurrency := getAsynqConcurrency()
	srv := asynq.NewServer(
		opts,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical":     6,
				"default":      3,
				"low":          1,
				"progress":     2,
				"gamification": 1,
				// ai_critical runs heavy AI work (exam generation) without
				// blocking the HTTP request thread. Weighted the same as
				// "critical" so multiple teachers generating exams at once
				// do not starve user-facing jobs.
				"ai_critical": 6,
			},
		},
	)

	mux := asynq.NewServeMux()

	notificationHandler := &NotificationHandler{}
	mux.HandleFunc(TypeMultiChannelNotification, WithTaskIdempotency(notificationHandler.ProcessTask))

	progressHandler := &ProgressHandler{}
	mux.HandleFunc(TypeProgressUpdate, WithTaskIdempotency(progressHandler.ProcessTask))

	gamificationHandler := &GamificationHandler{}
	mux.HandleFunc(TypeGamificationSync, WithTaskIdempotency(gamificationHandler.ProcessTask))

	batchFlushHandler := &BatchProgressFlushHandler{}
	mux.HandleFunc(TypeBatchProgressFlush, WithTaskIdempotency(batchFlushHandler.ProcessTask))

	cqrsRefreshHandler := &CQRSRefreshHandler{}
	mux.HandleFunc(TypeRefreshMaterializedViews, cqrsRefreshHandler.ProcessTask)

	sessionCleanupHandler := &SessionCleanupHandler{}
	mux.HandleFunc(TypeSessionCleanup, sessionCleanupHandler.ProcessTask)

	// Heavy AI exam generation. Runs in the worker pool, NOT in the Gin
	// request thread, so a single teacher generating 50 questions cannot
	// block every other user.
	examHandler := NewExamGenerationHandler()
	mux.HandleFunc(TypeExamGeneration, WithTaskIdempotency(examHandler.ProcessTask))

	// Essay grading and lesson summarization — same pattern as exam generation.
	// The HTTP handlers return 202 + jobId immediately; these workers do the
	// actual LLM call (5–30 s) without blocking any Gin goroutine.
	essayHandler := NewEssayGradeHandler()
	mux.HandleFunc(TypeEssayGrade, WithTaskIdempotency(essayHandler.ProcessTask))

	summaryHandler := NewLessonSummaryHandler()
	mux.HandleFunc(TypeLessonSummary, WithTaskIdempotency(summaryHandler.ProcessTask))

	// Drip content release notifications (Phase 3). Wrapped in
	// WithTaskIdempotency like every other handler above: this fans out a
	// notification to every enrolled student, so an asynq redelivery
	// (worker crash mid-run, or a retry after a transient DB error) would
	// otherwise re-send notifications to students already notified on the
	// first attempt.
	mux.HandleFunc(TypeDripContentRelease, WithTaskIdempotency(func(ctx context.Context, task *asynq.Task) error {
		return HandleDripContentRelease(task)
	}))

	// Course availability scheduling (Phase 3) — same redelivery-duplication
	// risk and fix as drip content release above.
	mux.HandleFunc(TypeCourseAvailability, WithTaskIdempotency(func(ctx context.Context, task *asynq.Task) error {
		return HandleCourseAvailability(task)
	}))

	// Periodic service-health probe (database, cache, storage, search, queue,
	// scheduler, api) — persists one row per service per run so admin health
	// detail pages can show real history. Not wrapped in WithTaskIdempotency:
	// it has no side effect beyond an insert keyed by (checked_at, service_key),
	// so a redelivered run just re-probes and inserts a fresh timestamp.
	healthCheckHandler := &HealthCheckHandler{}
	mux.HandleFunc(TypeServiceHealthCheck, healthCheckHandler.ProcessTask)

	log.Printf("[Worker] Asynq Server initialized (Concurrency=%d)", concurrency)

	// Run the server in a separate goroutine so we can listen for ctx.Done
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(mux)
	}()

	select {
	case <-ctx.Done():
		log.Println("[Worker] Shutdown signal received — stopping Asynq worker server...")
		srv.Shutdown()
		log.Println("[Worker] Asynq worker server stopped")
	case err := <-errCh:
		if err != nil {
			log.Printf("[Worker] Asynq worker server exited with error: %v", err)
		}
	}
}

// StartWorker is kept for backwards-compatibility; it runs until the process exits.
func StartWorker() {
	StartWorkerWithContext(context.Background())
}
