package worker

import (
	"os"
	"sync"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/hibiken/asynq"
)

var (
	client     *asynq.Client
	clientOnce sync.Once
)

// isRedisDisabled returns true when Redis should not be used or is unavailable.
func isRedisDisabled() bool {
	if os.Getenv("DISABLE_REDIS") == "true" {
		return true
	}
	if os.Getenv("VERCEL") != "" {
		return true
	}
	return false
}

// GetClient returns the asynq client, or nil if Redis is disabled/unavailable.
//
// CONCURRENCY: previously used a bare check-then-set on the package-level
// `client` variable, which is a data race under concurrent callers (the
// Enqueue* helpers below can be invoked from many request-handling
// goroutines at once) — multiple goroutines could all observe client == nil
// and each construct + assign their own *asynq.Client, leaking every
// instance but the last. sync.Once makes the construction happen exactly
// once regardless of concurrent callers.
func GetClient() *asynq.Client {
	clientOnce.Do(func() {
		redisAddr := os.Getenv("REDIS_URL")
		if redisAddr == "" || isRedisDisabled() {
			return
		}

		opts := cache.ParseAsynqRedisConnOpt(redisAddr)
		client = asynq.NewClient(opts)
	})
	return client
}

func EnqueueNotification(payload NotificationPayload) error {
	task, err := NewMultiChannelNotificationTask(payload)
	if err != nil {
		return err
	}

	cl := GetClient()
	if cl == nil {
		return nil
	}
	_, err = cl.Enqueue(task)
	return err
}

func EnqueueProgressUpdate(payload ProgressUpdatePayload) error {
	task, err := NewProgressUpdateTask(payload)
	if err != nil {
		return err
	}

	cl := GetClient()
	if cl == nil {
		return nil
	}
	_, err = cl.Enqueue(task, asynq.Queue("progress"), asynq.ProcessIn(5*time.Second))
	return err
}

func EnqueueGamificationSync(payload GamificationSyncPayload) error {
	task, err := NewGamificationSyncTask(payload)
	if err != nil {
		return err
	}

	cl := GetClient()
	if cl == nil {
		return nil
	}
	_, err = cl.Enqueue(task, asynq.Queue("gamification"), asynq.ProcessIn(5*time.Second))
	return err
}

func EnqueueBatchProgressFlush(userID string) error {
	payload := BatchProgressFlushPayload{UserID: userID}
	task, err := NewBatchProgressFlushTask(payload)
	if err != nil {
		return err
	}

	cl := GetClient()
	if cl == nil {
		return nil
	}
	_, err = cl.Enqueue(task, asynq.Queue("progress"), asynq.ProcessIn(5*time.Second))
	return err
}
