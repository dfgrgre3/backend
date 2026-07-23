package worker

import (
	"log"
	"os"
	"strings"

	"github.com/hibiken/asynq"
)

// StartScheduler starts the periodic task scheduler.
// It runs CQRS materialized view refresh every 5 minutes.
func StartScheduler() {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" || isRedisDisabled() {
		log.Println("[Scheduler] Redis not configured or disabled, skipping scheduler start")
		return
	}

	var opts asynq.RedisConnOpt
	if strings.HasPrefix(redisAddr, "redis://") || strings.HasPrefix(redisAddr, "rediss://") {
		parsedOpts, err := asynq.ParseRedisURI(redisAddr)
		if err != nil {
			log.Printf("failed to parse redis uri for scheduler: %v", err)
			return
		}
		opts = parsedOpts
	} else {
		opts = asynq.RedisClientOpt{Addr: redisAddr}
	}

	scheduler := asynq.NewScheduler(opts, &asynq.SchedulerOpts{})

	// Refresh materialized views every 5 minutes
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(TypeRefreshMaterializedViews, []byte("{}"))); err != nil {
		log.Printf("Failed to register CQRS refresh task: %v", err)
		return
	}

	// Purge expired sessions every 1 hour
	if _, err := scheduler.Register("@every 1h", asynq.NewTask(TypeSessionCleanup, []byte("{}"))); err != nil {
		log.Printf("Failed to register Session cleanup task: %v", err)
		return
	}

	// Check for drip content releases every 5 minutes (fallback for missed tasks)
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(TypeDripContentRelease, []byte("{\"check\":\"fallback\"}"))); err != nil {
		log.Printf("Failed to register Drip content check task: %v", err)
	}

	log.Println("[Scheduler] Periodic CQRS view refresh (5m), Session cleanup (1h), and Drip check (5m) scheduled")
	if err := scheduler.Start(); err != nil {
		log.Printf("Failed to start scheduler: %v", err)
	}
}
