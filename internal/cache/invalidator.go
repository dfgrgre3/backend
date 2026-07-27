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

type CacheInvalidator struct{}

func NewCacheInvalidator() *CacheInvalidator {
	return &CacheInvalidator{}
}

func (ci *CacheInvalidator) InvalidateSubject(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	key := fmt.Sprintf("subject:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "subj:list:*")
	log.Printf("[Cache] Invalidated subject cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateUser(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	ci.del(ctx, fmt.Sprintf("user:id:%s", id))
	ci.del(ctx, "user:email:*")
	log.Printf("[Cache] Invalidated user cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateCategory(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	key := fmt.Sprintf("cat:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "cat:list:*")
	log.Printf("[Cache] Invalidated category cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateExam(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	key := fmt.Sprintf("exam:id:%s", id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, "exam:list:*")
	log.Printf("[Cache] Invalidated exam cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateAllLists(ctx context.Context) {
	if db.Redis == nil {
		return
	}
	ci.invalidatePattern(ctx, "*:list:*")
	log.Printf("[Cache] Invalidated all list caches")
}

func (ci *CacheInvalidator) InvalidateMaterializedViews(ctx context.Context) {
	if db.Redis == nil {
		return
	}
	ci.del(ctx, "mv_user_progress_summary")
	ci.del(ctx, "mv_user_weekly_analytics")
	ci.del(ctx, "mv_user_watch_time")
	log.Printf("[Cache] Invalidated materialized view caches")
}

func (ci *CacheInvalidator) InvalidateTeacher(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	ci.invalidatePattern(ctx, "teacher:*")
	log.Printf("[Cache] Invalidated teacher cache: %s", id)
}

func (ci *CacheInvalidator) del(ctx context.Context, key string) {
	if err := db.Redis.Del(ctx, key).Err(); err != nil {
		log.Printf("[Cache] Error deleting key %s: %v", key, err)
	}
}

func (ci *CacheInvalidator) invalidatePattern(ctx context.Context, pattern string) {
	iter := db.Redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := db.Redis.Del(ctx, iter.Val()).Err(); err != nil {
			log.Printf("[Cache] Error deleting pattern match %s: %v", iter.Val(), err)
		}
	}
}