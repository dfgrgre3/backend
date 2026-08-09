package invalidate

import (
	"context"
	"fmt"
	"log"

	redisconn "thanawy-backend/internal/infrastructure/cache/redis"
)

// Invalidator removes stale cache entries from Redis.
// All methods are safe to call when Redis is unavailable — they simply no-op.
type Invalidator struct{}

func New() *Invalidator {
	return &Invalidator{}
}

func (ci *Invalidator) InvalidateSubject(ctx context.Context, id string) {
	if redisconn.Get() == nil {
		return
	}
	key := fmt.Sprintf("subject:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "subj:list:*")
	log.Printf("[Cache] Invalidated subject cache: %s", id)
}

func (ci *Invalidator) InvalidateUser(ctx context.Context, id string) {
	if redisconn.Get() == nil {
		return
	}
	// Delete the user-by-ID key directly (exact key, no pattern needed).
	ci.del(ctx, fmt.Sprintf("user:id:%s", id))
	// Invalidate all user-by-email keys via pattern scan.
	ci.invalidatePattern(ctx, "user:email:*")
	log.Printf("[Cache] Invalidated user cache: %s", id)
}

func (ci *Invalidator) InvalidateCategory(ctx context.Context, id string) {
	if redisconn.Get() == nil {
		return
	}
	key := fmt.Sprintf("cat:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "cat:list:*")
	log.Printf("[Cache] Invalidated category cache: %s", id)
}

func (ci *Invalidator) InvalidateExam(ctx context.Context, id string) {
	if redisconn.Get() == nil {
		return
	}
	key := fmt.Sprintf("exam:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "exam:list:*")
	log.Printf("[Cache] Invalidated exam cache: %s", id)
}

func (ci *Invalidator) InvalidateAllLists(ctx context.Context) {
	if redisconn.Get() == nil {
		return
	}
	ci.invalidatePattern(ctx, "*:list:*")
	log.Printf("[Cache] Invalidated all list caches")
}

func (ci *Invalidator) InvalidateMaterializedViews(ctx context.Context) {
	client := redisconn.Get()
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

func (ci *Invalidator) InvalidateTeacher(ctx context.Context, id string) {
	if redisconn.Get() == nil {
		return
	}
	ci.invalidatePattern(ctx, "teacher:*")
	log.Printf("[Cache] Invalidated teacher cache: %s", id)
}

func (ci *Invalidator) InvalidateNavigation(ctx context.Context) {
	if redisconn.Get() == nil {
		return
	}
	ci.invalidatePattern(ctx, "navigation:*")
	log.Printf("[Cache] Invalidated navigation cache")
}

// del deletes a single exact key from Redis. Uses the live client so it is
// always safe even if Redis reconnected after this invalidator was created.
func (ci *Invalidator) del(ctx context.Context, key string) {
	client := redisconn.Get()
	if client == nil {
		return
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		log.Printf("[Cache] Error deleting key %s: %v", key, err)
	}
}

// invalidatePattern scans for keys matching the given glob pattern and deletes
// them using a Redis Pipeline to minimise round-trips.
func (ci *Invalidator) invalidatePattern(ctx context.Context, pattern string) {
	client := redisconn.Get()
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
