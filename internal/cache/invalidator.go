package cache

import (
	"context"
	"fmt"
	"log"

	"thanawy-backend/internal/db"
)

// Cache key format patterns
const (
	entityIDKeyFmt = "%sid:%s"
)

// CacheInvalidator removes stale cache entries from Redis.
// All methods are safe to call when Redis is unavailable — they simply no-op.
type CacheInvalidator struct{}

func NewCacheInvalidator() *CacheInvalidator {
	return &CacheInvalidator{}
}

func (ci *CacheInvalidator) InvalidateSubject(ctx context.Context, id string) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	key := fmt.Sprintf("subject:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "subj:list:*")
	log.Printf("[Cache] Invalidated subject cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateUser(ctx context.Context, id string) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	// Delete the user-by-ID key directly (exact key, no pattern needed).
	ci.del(ctx, fmt.Sprintf("user:id:%s", id))
	// Invalidate all user-by-email keys via pattern scan (previously this was
	// calling del("user:email:*") which deleted a literal key named "user:email:*"
	// rather than scanning matching keys — this is the bug fix).
	ci.invalidatePattern(ctx, "user:email:*")
	log.Printf("[Cache] Invalidated user cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateCategory(ctx context.Context, id string) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	key := fmt.Sprintf("cat:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "cat:list:*")
	log.Printf("[Cache] Invalidated category cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateExam(ctx context.Context, id string) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	key := fmt.Sprintf("exam:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "exam:list:*")
	log.Printf("[Cache] Invalidated exam cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateAllLists(ctx context.Context) {
	if db.GetRedis() == nil {
		return
	}
	ci.invalidatePattern(ctx, "*:list:*")
	log.Printf("[Cache] Invalidated all list caches")
}

func (ci *CacheInvalidator) InvalidateMaterializedViews(ctx context.Context) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	pipe := client.Pipeline()
	pipe.Del(ctx, "mv_user_progress_summary")
	pipe.Del(ctx, "mv_user_weekly_analytics")
	pipe.Del(ctx, "mv_user_watch_time")
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[Cache] Error invalidating materialized view caches: %v", err)
	}
	log.Printf("[Cache] Invalidated materialized view caches")
}

func (ci *CacheInvalidator) InvalidateTeacher(ctx context.Context, id string) {
	if db.GetRedis() == nil {
		return
	}
	ci.invalidatePattern(ctx, "teacher:*")
	log.Printf("[Cache] Invalidated teacher cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateNavigation(ctx context.Context) {
	if db.GetRedis() == nil {
		return
	}
	ci.invalidatePattern(ctx, "navigation:*")
	log.Printf("[Cache] Invalidated navigation cache")
}

// del deletes a single exact key from Redis. Uses the live client so it is
// always safe even if Redis reconnected after this invalidator was created.
func (ci *CacheInvalidator) del(ctx context.Context, key string) {
	client := db.GetRedis()
	if client == nil {
		return
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		log.Printf("[Cache] Error deleting key %s: %v", key, err)
	}
}

// invalidatePattern scans for keys matching the given glob pattern and deletes
// them using a Redis Pipeline to minimise round-trips.
func (ci *CacheInvalidator) invalidatePattern(ctx context.Context, pattern string) {
	client := db.GetRedis()
	if client == nil {
		return
	}

	iter := client.Scan(ctx, 0, pattern, 100).Iterator()
	pipe := client.Pipeline()
	count := 0
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
		count++
	}
	if err := iter.Err(); err != nil {
		log.Printf("[Cache] SCAN error for pattern %s: %v", pattern, err)
	}
	if count == 0 {
		return
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[Cache] Pipeline DEL error for pattern %s: %v", pattern, err)
	}
}
