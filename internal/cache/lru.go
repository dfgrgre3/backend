package cache

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

type LRUCache struct {
	mu       sync.Mutex
	items    map[string]*cacheEntry
	maxSize  int
	keysList []string
}

func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		items:   make(map[string]*cacheEntry),
		maxSize: maxSize,
	}
}

func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing key
	if _, exists := c.items[key]; exists {
		c.items[key] = &cacheEntry{
			value:     value,
			expiresAt: time.Now().Add(ttl),
		}
		c.removeKeyFromList(key)
		c.keysList = append(c.keysList, key)
		return
	}

	// Evict oldest if capacity is reached
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.keysList = append(c.keysList, key)
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		delete(c.items, key)
		c.removeKeyFromList(key)
		return nil, false
	}

	// Touch/move key to end to keep it fresh in FIFO/LRU sense
	c.removeKeyFromList(key)
	c.keysList = append(c.keysList, key)

	return entry.value, true
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	c.removeKeyFromList(key)
}

func (c *LRUCache) removeKeyFromList(key string) {
	for i, k := range c.keysList {
		if k == key {
			c.keysList = append(c.keysList[:i], c.keysList[i+1:]...)
			break
		}
	}
}

func (c *LRUCache) evictOldest() {
	if len(c.keysList) == 0 {
		return
	}
	oldestKey := c.keysList[0]
	c.keysList = c.keysList[1:]
	delete(c.items, oldestKey)
}
