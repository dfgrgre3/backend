package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"thanawy-backend/internal/infrastructure/cache"


	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// NewExamGenerationTask creates an asynq task from the payload.
func NewExamGenerationTask(payload ExamGenerationPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExamGeneration, data,
		asynq.MaxRetry(2),
		asynq.Timeout(180*time.Second),
	), nil
}

// EnqueueExamGeneration enqueues the job and returns the jobId.
// If Redis is unavailable it returns ("", nil) so the caller can fall back
// to the synchronous path.
func EnqueueExamGeneration(payload ExamGenerationPayload) (string, error) {
	if payload.JobID == "" {
		payload.JobID = uuid.New().String()
	}
	task, err := NewExamGenerationTask(payload)
	if err != nil {
		return "", err
	}

	cl := GetClient()
	if cl == nil {
		return "", nil // Redis unavailable → caller must run synchronously
	}

	if _, err := cl.Enqueue(task, asynq.Queue("ai_critical")); err != nil {
		return "", fmt.Errorf("enqueue exam generation task: %w", err)
	}
	return payload.JobID, nil
}

// SetExamJobResult stores the job state in Redis so the polling endpoint can
// read it. Silently no-ops if Redis is unavailable.
func SetExamJobResult(ctx context.Context, result ExamGenerationResult) {
	if cache.Redis == nil {
		return
	}
	if result.JobID == "" {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ExamJob] failed to marshal result for %s: %v", result.JobID, err)
		return
	}
	if err := cache.Redis.Set(ctx, examResultKeyPrefix+result.JobID, data, examResultTTL).Err(); err != nil {
		log.Printf("[ExamJob] failed to write redis result for %s: %v", result.JobID, err)
	}
}

// GetExamJobResult reads the current state of a job from Redis.
// Returns (nil, false) if the key does not exist or Redis is down.
func GetExamJobResult(ctx context.Context, jobID string) (*ExamGenerationResult, bool) {
	if cache.Redis == nil || jobID == "" {
		return nil, false
	}
	raw, err := cache.Redis.Get(ctx, examResultKeyPrefix+jobID).Result()
	if err != nil {
		return nil, false
	}
	var out ExamGenerationResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	return &out, true
}
