package cache

import (
	"thanawy-backend/internal/infrastructure/cache/lru"
)

// Compatibility facade over the cache/lru package.
// New code should import cache/lru directly.

// LRUCache is a thread-safe, fixed-capacity LRU cache with per-entry TTL.
type LRUCache = lru.Cache

func NewLRUCache(maxSize int) *LRUCache { return lru.New(maxSize) }
