package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"thanawy-backend/internal/db"
)

// EnhancedCache provides a two-level cache:
//   - L1: in-process LRU cache (fast, no network round-trip)
//   - L2: Redis (shared across replicas, survives restarts)
//
// The Redis client is resolved lazily via db.GetRedis() on every operation so
// that the cache remains fully functional even if Redis connects after the
// cache is first created (common in production startup ordering).
type EnhancedCache struct {
	local *LRUCache
	stats *CacheStats
	mu    sync.RWMutex
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	mu        sync.Mutex
	hits      int64
	misses    int64
	errors    int64
	localHits int64
	redisHits int64
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

// NewEnhancedCache creates a new enhanced cache instance.
// maxLocalSize controls the in-process LRU capacity.
func NewEnhancedCache(maxLocalSize int) *EnhancedCache {
	return &EnhancedCache{
		local: NewLRUCache(maxLocalSize),
		stats: &CacheStats{},
	}
}

// Get retrieves a value using read-through strategy (L1 → L2 → miss).
func (ec *EnhancedCache) Get(ctx context.Context, key string, dest interface{}) error {
	// Try local cache first (L1)
	if ec.local != nil {
		if value, ok := ec.local.Get(key); ok {
			ec.recordLocalHit()
			return json.Unmarshal(value.([]byte), dest)
		}
	}

	// Try Redis cache (L2)
	if client := db.GetRedis(); client != nil {
		data, err := client.Get(ctx, key).Bytes()
		if err == nil {
			ec.recordRedisHit()
			// Populate local cache for subsequent reads.
			if ec.local != nil {
				ec.local.Set(key, data, 5*time.Minute)
			}
			return json.Unmarshal(data, dest)
		}
	}

	ec.recordMiss()
	return fmt.Errorf("cache miss: %s", key)
}

// Set stores a value using write-through strategy (L1 and L2 simultaneously).
func (ec *EnhancedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		ec.recordError()
		return err
	}

	// Write to local cache.
	if ec.local != nil {
		ec.local.Set(key, data, ttl)
	}

	// Write to Redis.
	if client := db.GetRedis(); client != nil {
		if err := client.Set(ctx, key, data, ttl).Err(); err != nil {
			ec.recordError()
			log.Printf("[Cache] Redis set error for key %s: %v", key, err)
			return err
		}
	}

	return nil
}

// Delete removes a key from all cache layers.
func (ec *EnhancedCache) Delete(ctx context.Context, key string) error {
	if ec.local != nil {
		ec.local.Delete(key)
	}
	if client := db.GetRedis(); client != nil {
		return client.Del(ctx, key).Err()
	}
	return nil
}

// InvalidatePattern removes keys matching a pattern from Redis and clears the
// entire local cache (pattern scanning local cache is not cost-effective).
func (ec *EnhancedCache) InvalidatePattern(ctx context.Context, pattern string) error {
	if client := db.GetRedis(); client != nil {
		iter := client.Scan(ctx, 0, pattern, 100).Iterator()
		pipe := client.Pipeline()
		count := 0
		for iter.Next(ctx) {
			pipe.Del(ctx, iter.Val())
			count++
		}
		if count > 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("[Cache] Error executing pipeline delete for pattern %s: %v", pattern, err)
			}
		}
		if err := iter.Err(); err != nil {
			return err
		}
	}
	// Clear local cache on pattern invalidation — scanning all local keys is
	// O(n) and not worth it; a full local clear is cheaper and safe.
	if ec.local != nil {
		ec.local.Clear()
	}
	return nil
}

// GetOrSet implements cache-aside: return the cached value if available,
// otherwise call loader, store the result, and return it.
func (ec *EnhancedCache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func() (interface{}, error)) error {
	// Try to get from cache.
	if err := ec.Get(ctx, key, dest); err == nil {
		return nil
	}

	// Cache miss — load data from the source.
	value, err := loader()
	if err != nil {
		return err
	}

	// Marshal once, reuse for both store and unmarshal into dest.
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Store in both cache layers (best-effort — don't fail the caller on cache write errors).
	if ec.local != nil {
		ec.local.Set(key, data, ttl)
	}
	if client := db.GetRedis(); client != nil {
		if err := client.Set(ctx, key, data, ttl).Err(); err != nil {
			log.Printf("[Cache] Failed to store loaded value in Redis: %v", err)
		}
	}

	return json.Unmarshal(data, dest)
}

// GetStats returns cache performance statistics.
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

// CacheStatsSnapshot is a point-in-time view of cache stats.
type CacheStatsSnapshot struct {
	TotalRequests int64
	Hits          int64
	Misses        int64
	Errors        int64
	LocalHits     int64
	RedisHits     int64
	HitRate       float64
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

// Global enhanced cache singleton.
// Uses sync.Once to create exactly one instance. The instance itself always
// resolves the live Redis client at call time, so it is safe to call
// GetEnhancedCache() before Redis has connected.
var (
	GlobalEnhancedCache *EnhancedCache
	cacheOnce           sync.Once
)

func GetEnhancedCache() *EnhancedCache {
	cacheOnce.Do(func() {
		GlobalEnhancedCache = NewEnhancedCache(10000) // 10k local entries
	})
	return GlobalEnhancedCache
}
