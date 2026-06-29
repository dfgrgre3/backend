package worker

import (
	"log"
	"os"
	"strings"

	"github.com/hibiken/asynq"
)

func StartWorker() {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" || isRedisDisabled() {
		log.Println("[Worker] Redis not configured or disabled, skipping worker start")
		return
	}

	var opts asynq.RedisConnOpt
	if strings.HasPrefix(redisAddr, "redis://") || strings.HasPrefix(redisAddr, "rediss://") {
		parsedOpts, err := asynq.ParseRedisURI(redisAddr)
		if err != nil {
			log.Printf("failed to parse redis uri: %v — workers will not start", err)
			return
		}
		opts = parsedOpts
	} else {
		opts = asynq.RedisClientOpt{Addr: redisAddr}
	}

	srv := asynq.NewServer(
		opts,
		asynq.Config{
			Concurrency: 10,
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

	log.Printf("Worker server starting on Redis %s", redisAddr)
	if err := srv.Run(mux); err != nil {
		log.Printf("could not run worker server: %v", err)
	}
}
