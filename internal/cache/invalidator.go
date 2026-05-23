package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"thanawy-backend/internal/db"
)

const (
	CachePrefixSubject  = "subject:"
	CachePrefixUser     = "user:"
	CachePrefixCategory = "category:"
	CachePrefixExam     = "exam:"
	CachePrefixList     = "list:"
	CacheTTLSubject     = 30 * time.Minute
	CacheTTLUser        = 15 * time.Minute
	CacheTTLCategory    = 1 * time.Hour
	CacheTTLExam        = 30 * time.Minute
	CacheTTLList        = 5 * time.Minute
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
	key := fmt.Sprintf(entityIDKeyFmt, CachePrefixSubject, id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, CachePrefixSubject + CachePrefixList + "*")
	log.Printf("[Cache] Invalidated subject cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateUser(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	ci.del(ctx, fmt.Sprintf(entityIDKeyFmt, CachePrefixUser, id))
	ci.del(ctx, fmt.Sprintf("%semail:*", CachePrefixUser))
	log.Printf("[Cache] Invalidated user cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateCategory(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	key := fmt.Sprintf(entityIDKeyFmt, CachePrefixCategory, id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, CachePrefixCategory + CachePrefixList + "*")
	log.Printf("[Cache] Invalidated category cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateExam(ctx context.Context, id string) {
	if db.Redis == nil {
		return
	}
	key := fmt.Sprintf(entityIDKeyFmt, CachePrefixExam, id)
	ci.del(ctx, key)
	ci.invalidatePattern(ctx, CachePrefixExam + CachePrefixList + "*")
	log.Printf("[Cache] Invalidated exam cache: %s", id)
}

func (ci *CacheInvalidator) InvalidateAllLists(ctx context.Context) {
	if db.Redis == nil {
		return
	}
	ci.invalidatePattern(ctx, "*" + CachePrefixList + "*")
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