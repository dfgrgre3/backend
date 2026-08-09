package settings

import (
	"sync"
	"time"
)

var (
	mu       sync.RWMutex
	cached   map[string]interface{}
	cachedAt time.Time
	cacheTTL = 30 * time.Second
)

// Invalidate drops the cached admin settings so the next read
// reloads them from the database.
func Invalidate() {
	mu.Lock()
	cached = nil
	mu.Unlock()
}

// Get returns the cached admin settings and whether they are still fresh.
func Get() (map[string]interface{}, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if cached != nil && time.Since(cachedAt) < cacheTTL {
		return cached, true
	}
	return nil, false
}

// Store caches the given admin settings.
func Store(s map[string]interface{}) {
	mu.Lock()
	cached = s
	cachedAt = time.Now()
	mu.Unlock()
}
