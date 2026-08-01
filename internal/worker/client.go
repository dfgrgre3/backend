package worker

import (
	"os"
	"time"

	"github.com/hibiken/asynq"
)

var client *asynq.Client

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
func GetClient() *asynq.Client {
	if client == nil {
		redisAddr := os.Getenv("REDIS_URL")
		if redisAddr == "" || isRedisDisabled() {
			return nil
		}

		opts := parseAsynqRedisConnOpt(redisAddr)

		client = asynq.NewClient(opts)
	}
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
