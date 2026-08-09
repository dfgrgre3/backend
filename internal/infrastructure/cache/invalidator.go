package cache

import (
	"thanawy-backend/internal/infrastructure/cache/invalidate"
)

// Compatibility facade over the cache/invalidate package.
// New code should import cache/invalidate directly.

// CacheInvalidator removes stale cache entries from Redis.
type CacheInvalidator = invalidate.Invalidator

func NewCacheInvalidator() *CacheInvalidator { return invalidate.New() }
