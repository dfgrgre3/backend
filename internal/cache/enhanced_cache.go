package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"thanawy-backend/internal/db"
)

// EnhancedCache provides advanced caching with multiple strategies
type EnhancedCache struct {
	redis *RedisCache
	local *LRUCache
	stats *CacheStats
	mu    sync.RWMutex
}

// RedisCache wraps Redis operations
type RedisCache struct {
	client *redis.Client
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	mu           sync.Mutex
	hits         int64
	misses       int64
	errors       int64
	localHits    int64
	redisHits    int64
}

// CacheStrategy defines caching behavior
type CacheStrategy int

const (
	// CacheStrategyWriteThrough writes to both local and Redis
	CacheStrategyWriteThrough CacheStrategy = iota
	// CacheStrategyWriteBehind writes to local first, async to Redis
	CacheStrategyWriteBehind
	// CacheStrategyReadThrough reads from local, falls back to Redis
	CacheStrategyReadThrough
	// CacheStrategyRefreshAhead proactively refreshes expiring keys
	CacheStrategyRefreshAhead
)

// NewEnhancedCache creates a new enhanced cache instance
func NewEnhancedCache(maxLocalSize int) *EnhancedCache {
	return &EnhancedCache{
		redis: &RedisCache{client: db.Redis},
		local: NewLRUCache(maxLocalSize),
		stats: &CacheStats{},
	}
}

// Get retrieves a value using read-through strategy
func (ec *EnhancedCache) Get(ctx context.Context, key string, dest interface{}) error {
	// Try local cache first (L1)
	if ec.local != nil {
		if value, ok := ec.local.Get(key); ok {
			ec.recordLocalHit()
			return json.Unmarshal(value.([]byte), dest)
		}
	}

	// Try Redis cache (L2)
	if ec.redis.client != nil {
		data, err := ec.redis.client.Get(ctx, key).Bytes()
		if err == nil {
			ec.recordRedisHit()
			// Populate local cache
			if ec.local != nil {
				ec.local.Set(key, data, 5*time.Minute)
			}
			return json.Unmarshal(data, dest)
		}
	}

	ec.recordMiss()
	return fmt.Errorf("cache miss: %s", key)
}

// Set stores a value using write-through strategy
func (ec *EnhancedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		ec.recordError()
		return err
	}

	// Write to local cache
	if ec.local != nil {
		ec.local.Set(key, data, ttl)
	}

	// Write to Redis
	if ec.redis.client != nil {
		if err := ec.redis.client.Set(ctx, key, data, ttl).Err(); err != nil {
			ec.recordError()
			log.Printf("[Cache] Redis set error for key %s: %v", key, err)
			return err
		}
	}

	return nil
}

// Delete removes a key from all cache layers
func (ec *EnhancedCache) Delete(ctx context.Context, key string) error {
	if ec.local != nil {
		ec.local.Delete(key)
	}
	if ec.redis.client != nil {
		return ec.redis.client.Del(ctx, key).Err()
	}
	return nil
}

// InvalidatePattern removes keys matching a pattern
func (ec *EnhancedCache) InvalidatePattern(ctx context.Context, pattern string) error {
	if ec.redis.client != nil {
		iter := ec.redis.client.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			if err := ec.redis.client.Del(ctx, iter.Val()).Err(); err != nil {
				log.Printf("[Cache] Error deleting key %s: %v", iter.Val(), err)
			}
		}
	}
	// Note: Local cache pattern invalidation would require scanning all keys
	// For simplicity, we clear local cache on pattern invalidation
	if ec.local != nil {
		ec.local.Clear()
	}
	return nil
}

// GetOrSet implements cache-aside pattern with automatic population
func (ec *EnhancedCache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func() (interface{}, error)) error {
	// Try to get from cache
	err := ec.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// Cache miss - load data
	value, err := loader()
	if err != nil {
		return err
	}

	// Store in cache
	if err := ec.Set(ctx, key, value, ttl); err != nil {
		log.Printf("[Cache] Failed to store loaded value: %v", err)
	}

	// Unmarshal into dest
	data, _ := json.Marshal(value)
	return json.Unmarshal(data, dest)
}

// GetStats returns cache performance statistics
func (ec *EnhancedCache) GetStats() CacheStatsSnapshot {
	ec.stats.mu.Lock()
	defer ec.stats.mu.Unlock()

	total := ec.stats.hits + ec.stats.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(ec.stats.hits) / float64(total) * 100
	}

	return CacheStatsSnapshot{
		TotalRequests: total,
		Hits:          ec.stats.hits,
		Misses:        ec.stats.misses,
		Errors:        ec.stats.errors,
		LocalHits:     ec.stats.localHits,
		RedisHits:     ec.stats.redisHits,
		HitRate:       hitRate,
	}
}

// CacheStatsSnapshot is a point-in-time view of cache stats
type CacheStatsSnapshot struct {
	TotalRequests int64
	Hits          int64
	Misses        int64
	Errors        int64
	LocalHits     int64
	RedisHits     int64
	HitRate       float64
}

func (ec *EnhancedCache) recordHit() {
	ec.stats.mu.Lock()
	ec.stats.hits++
	ec.stats.mu.Unlock()
}

func (ec *EnhancedCache) recordLocalHit() {
	ec.stats.mu.Lock()
	ec.stats.hits++
	ec.stats.localHits++
	ec.stats.mu.Unlock()
}

func (ec *EnhancedCache) recordRedisHit() {
	ec.stats.mu.Lock()
	ec.stats.hits++
	ec.stats.redisHits++
	ec.stats.mu.Unlock()
}

func (ec *EnhancedCache) recordMiss() {
	ec.stats.mu.Lock()
	ec.stats.misses++
	ec.stats.mu.Unlock()
}

func (ec *EnhancedCache) recordError() {
	ec.stats.mu.Lock()
	ec.stats.errors++
	ec.stats.mu.Unlock()
}

// Cache warming functions
func (ec *EnhancedCache) WarmSubjectCache(ctx context.Context, subjectIDs []string) error {
	for _, id := range subjectIDs {
		// Pre-load subject details
		key := SubjectDetailKey(id)
		// This would typically call a loader function
		_ = key
	}
	return nil
}

func (ec *EnhancedCache) WarmUserCache(ctx context.Context, userIDs []string) error {
	for _, id := range userIDs {
		key := UserProfileKey(id)
		_ = key
	}
	return nil
}

// Global enhanced cache instance
var GlobalEnhancedCache *EnhancedCache
var cacheOnce sync.Once

func GetEnhancedCache() *EnhancedCache {
	cacheOnce.Do(func() {
		GlobalEnhancedCache = NewEnhancedCache(10000) // 10k local entries
	})
	return GlobalEnhancedCache
}
