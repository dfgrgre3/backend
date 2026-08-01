package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/events"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	analyticsStream        = "analytics:events"
	analyticsConsumerGroup = "analytics-workers"
	analyticsConsumerID    = "worker-1"
	analyticsBatchSize     = 100
	analyticsBlockTime     = 5 * time.Second
)

// StartAnalyticsBatchWorkerWithContext starts the analytics event consumer.
// It prefers Redis Streams (Redis >= 5.0) but falls back to Redis Lists
// (LPUSH/BRPOP) for older Redis versions (3.x, 4.x).
// This runs in its own goroutine and is completely independent from Asynq.
// The loop exits cleanly when ctx is cancelled.
func StartAnalyticsBatchWorkerWithContext(ctx context.Context) {
	if !db.IsRedisAvailable() {
		log.Println("[AnalyticsWorker] Redis not available, skipping")
		return
	}

	// Try Redis Streams first (requires Redis >= 5.0)
	if err := ensureConsumerGroup(); err != nil {
		log.Printf("[AnalyticsWorker] Redis Streams not available (%v), falling back to Redis List mode", err)
		startListBasedWorkerWithContext(ctx)
		return
	}

	log.Println("[AnalyticsWorker] Starting Redis Stream consumer (separate from Asynq)")
	for {
		select {
		case <-ctx.Done():
			log.Println("[AnalyticsWorker] Shutdown signal received — stopping stream consumer")
			return
		default:
		}
		if err := processBatch(ctx); err != nil {
			log.Printf("[AnalyticsWorker] Batch error: %v", err)
			// Back off briefly on transient errors so we don't spin-loop.
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}

// StartAnalyticsBatchWorker is kept for backwards-compatibility.
func StartAnalyticsBatchWorker() {
	StartAnalyticsBatchWorkerWithContext(context.Background())
}

// startListBasedWorkerWithContext uses Redis Lists (LPUSH/BRPOP) as a fallback for Redis < 5.0.
// This provides the same analytics batching functionality without Redis Streams.
// The loop exits cleanly when ctx is cancelled.
func startListBasedWorkerWithContext(ctx context.Context) {
	log.Println("[AnalyticsWorker] Starting Redis List-based analytics consumer (fallback mode)")

	for {
		select {
		case <-ctx.Done():
			log.Println("[AnalyticsWorker] Shutdown signal received — stopping list consumer")
			return
		default:
		}
		// BRPOP blocks until an element is available (timeout: 5 seconds)
		result, err := db.GetRedis().BRPop(ctx, 5*time.Second, analyticsStream).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				// Timeout or context cancelled — loop to re-check ctx
				continue
			}
			log.Printf("[AnalyticsWorker] BRPOP error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// result[0] = key name, result[1] = value
		if len(result) < 2 {
			continue
		}
		rawData := result[1]

		// Parse the event
		var event events.AnalyticsEvent
		if err := json.Unmarshal([]byte(rawData), &event); err != nil {
			log.Printf("[AnalyticsWorker] Skipping malformed event: %v", err)
			continue
		}

		// Build and insert the record
		var payload map[string]interface{}
		json.Unmarshal(event.Payload, &payload)
		if payload == nil {
			payload = make(map[string]interface{})
		}

		record := map[string]interface{}{
			"event_id":    event.ID,
			"event_type":  string(event.Type),
			"user_id":     event.UserID,
			"payload":     payload,
			"source":      "frontend",
			"received_at": time.UnixMilli(event.Timestamp),
			"created_at":  time.Now().UTC(),
		}

		if err := batchInsert(ctx, []map[string]interface{}{record}); err != nil {
			log.Printf("[AnalyticsWorker] Failed to insert event %s: %v", event.ID, err)
		}
	}
}

func ensureConsumerGroup() error {
	err := db.GetRedis().XGroupCreateMkStream(
		context.Background(),
		analyticsStream,
		analyticsConsumerGroup,
		"$",
	).Err()
	if err != nil {
		// Check if the error is due to Redis Streams not being supported
		if strings.Contains(strings.ToLower(err.Error()), "unknown command") {
			log.Println("[AnalyticsWorker] Redis Streams not supported (Redis version < 5.0 or Streams disabled). Analytics batch worker disabled.")
			return fmt.Errorf("redis streams not supported")
		}
		// If the group already exists, that's fine
		if !hasPrefix(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func processBatch(ctx context.Context) error {
	messages, err := readBatchMessages(ctx)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	records, ids := buildAnalyticsRecords(messages)
	if len(records) > 0 {
		if err := batchInsert(ctx, records); err != nil {
			return err
		}
	}

	if err := db.GetRedis().XAck(ctx, analyticsStream, analyticsConsumerGroup, ids...).Err(); err != nil {
		log.Printf("[AnalyticsWorker] Failed to ack messages: %v", err)
		return err
	}

	return nil
}

func readBatchMessages(ctx context.Context) ([]redis.XMessage, error) {
	messages, err := readPendingAnalyticsMessages(ctx)
	if err != nil {
		return nil, err
	}
	if len(messages) > 0 {
		return messages, nil
	}
	return readNewAnalyticsMessages(ctx)
}

func readPendingAnalyticsMessages(ctx context.Context) ([]redis.XMessage, error) {
	entries, err := readAnalyticsGroupWithRetry(ctx, &redis.XReadGroupArgs{
		Group:    analyticsConsumerGroup,
		Consumer: analyticsConsumerID,
		Streams:  []string{analyticsStream, "0"},
		Count:    int64(analyticsBatchSize),
		Block:    0,
		NoAck:    false,
	})
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	messages := streamMessages(entries)
	if len(messages) > 0 {
		log.Printf("[AnalyticsWorker] Processing batch of %d pending (unacknowledged) events", len(messages))
	}
	return messages, nil
}

func readNewAnalyticsMessages(ctx context.Context) ([]redis.XMessage, error) {
	entries, err := readAnalyticsGroupWithRetry(ctx, &redis.XReadGroupArgs{
		Group:    analyticsConsumerGroup,
		Consumer: analyticsConsumerID,
		Streams:  []string{analyticsStream, ">"},
		Count:    int64(analyticsBatchSize),
		Block:    analyticsBlockTime,
		NoAck:    false,
	})
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	messages := streamMessages(entries)
	if len(messages) > 0 {
		log.Printf("[AnalyticsWorker] Processing batch of %d new events", len(messages))
	}
	return messages, nil
}

func streamMessages(entries []redis.XStream) []redis.XMessage {
	if len(entries) == 0 || len(entries[0].Messages) == 0 {
		return nil
	}
	return entries[0].Messages
}

func buildAnalyticsRecords(messages []redis.XMessage) ([]map[string]interface{}, []string) {
	records := make([]map[string]interface{}, 0, len(messages))
	ids := make([]string, 0, len(messages))

	for _, msg := range messages {
		if record, ok := analyticsRecordFromMessage(msg); ok {
			records = append(records, record)
		}
		ids = append(ids, msg.ID)
	}

	return records, ids
}

func analyticsRecordFromMessage(msg redis.XMessage) (map[string]interface{}, bool) {
	rawData, ok := msg.Values["data"].(string)
	if !ok {
		return nil, false
	}

	var event events.AnalyticsEvent
	if err := json.Unmarshal([]byte(rawData), &event); err != nil {
		log.Printf("[AnalyticsWorker] Skipping malformed event %s: %v", msg.ID, err)
		return nil, false
	}

	var payload map[string]interface{}
	json.Unmarshal(event.Payload, &payload)
	if payload == nil {
		payload = make(map[string]interface{})
	}

	return map[string]interface{}{
		"event_id":    event.ID,
		"event_type":  string(event.Type),
		"user_id":     event.UserID,
		"payload":     payload,
		"source":      "frontend",
		"received_at": time.UnixMilli(event.Timestamp),
		"created_at":  time.Now().UTC(),
	}, true
}

func readAnalyticsGroupWithRetry(ctx context.Context, args *redis.XReadGroupArgs) ([]redis.XStream, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		entries, err := db.GetRedis().XReadGroup(ctx, args).Result()
		if err == nil || err == redis.Nil {
			return entries, err
		}

		lastErr = err
		backoff := time.Duration(200*(attempt+1)) * time.Millisecond
		log.Printf("[AnalyticsWorker] Redis read attempt %d failed: %v; retrying in %s", attempt+1, err, backoff)
		time.Sleep(backoff)
	}
	return nil, lastErr
}

func batchInsert(ctx context.Context, records []map[string]interface{}) error {
	if db.WriteDB() == nil {
		return nil
	}

	batchSize := 50
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		tx := db.WriteDB().WithContext(ctx).Session(&gorm.Session{PrepareStmt: false}).Begin()
		stmt := `INSERT INTO "AnalyticsEvent" ("event_id", "event_type", "user_id", "payload", "source", "received_at", "created_at")
				 VALUES (?, ?, ?::text, ?::jsonb, ?, ?, ?)
				 ON CONFLICT ("event_id") DO NOTHING`
		for _, rec := range batch {
			if err := tx.Exec(stmt,
				rec["event_id"],
				rec["event_type"],
				rec["user_id"],
				toJSONString(rec["payload"]),
				rec["source"],
				rec["received_at"],
				rec["created_at"],
			).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit().Error; err != nil {
			return err
		}
	}

	log.Printf("[AnalyticsWorker] Inserted %d events into AnalyticsEvent", len(records))
	return nil
}

func toJSONString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
