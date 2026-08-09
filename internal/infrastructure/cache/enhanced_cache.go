package cache

import (
	"thanawy-backend/internal/infrastructure/cache/enhanced"
)

// Compatibility facade over the cache/enhanced package.
// New code should import cache/enhanced directly.

// EnhancedCache is a two-level (local LRU + Redis) cache.
type EnhancedCache = enhanced.Cache

// CacheStatsSnapshot is a point-in-time view of cache stats.
type CacheStatsSnapshot = enhanced.StatsSnapshot

// CacheStrategy defines caching behavior.
type CacheStrategy = enhanced.Strategy

const (
	CacheStrategyWriteThrough = enhanced.StrategyWriteThrough
	CacheStrategyWriteBehind  = enhanced.StrategyWriteBehind
	CacheStrategyReadThrough  = enhanced.StrategyReadThrough
	CacheStrategyRefreshAhead = enhanced.StrategyRefreshAhead
)

func NewEnhancedCache(maxLocalSize int) *EnhancedCache { return enhanced.New(maxLocalSize) }

func GetEnhancedCache() *EnhancedCache { return enhanced.Global() }
