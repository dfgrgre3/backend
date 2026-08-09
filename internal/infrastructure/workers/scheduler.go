package worker

import (
	"log"
	"os"
	"thanawy-backend/internal/infrastructure/cache"

	"github.com/hibiken/asynq"
)

// StartScheduler starts the periodic task scheduler.
// It runs CQRS materialized view refresh every 5 minutes.
// Cron expressions that include a time-of-day are evaluated in the local
// timezone returned by SchedulerLocation() (default: Africa/Cairo).
func StartScheduler() {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" || isRedisDisabled() {
		log.Println("[Scheduler] Redis not configured or disabled, skipping scheduler start")
		return
	}

	opts := cache.ParseAsynqRedisConnOpt(redisAddr)

	loc := SchedulerLocation()

	scheduler := asynq.NewScheduler(opts, &asynq.SchedulerOpts{
		// All cron expressions with a time-of-day component (e.g. "0 9 * * *")
		// are interpreted in this timezone, so jobs fire at the correct local
		// time regardless of the server's system clock.
		Location: loc,
	})

	// Refresh materialized views every 5 minutes (interval — timezone-agnostic)
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(TypeRefreshMaterializedViews, []byte("{}"))); err != nil {
		log.Printf("Failed to register CQRS refresh task: %v", err)
		return
	}

	// Purge expired sessions every 1 hour (interval — timezone-agnostic)
	if _, err := scheduler.Register("@every 1h", asynq.NewTask(TypeSessionCleanup, []byte("{}"))); err != nil {
		log.Printf("Failed to register Session cleanup task: %v", err)
		return
	}

	// Check for drip content releases every 5 minutes (fallback for missed tasks)
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(TypeDripContentRelease, []byte("{\"check\":\"fallback\"}"))); err != nil {
		log.Printf("Failed to register Drip content check task: %v", err)
	}

	log.Printf("[Scheduler] Periodic tasks scheduled (timezone: %s): CQRS refresh (5m), Session cleanup (1h), Drip check (5m)", loc)
	if err := scheduler.Start(); err != nil {
		log.Printf("Failed to start scheduler: %v", err)
	}
}
