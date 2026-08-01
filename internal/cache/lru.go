package cache

import (
	"container/list"
	"sync"
	"time"
)

// cacheEntry holds a cached value alongside its expiry time and a pointer to its
// position in the eviction list so that Get/Delete can remove it in O(1).
type cacheEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
	element   *list.Element // pointer back into evictionList
}

// LRUCache is a thread-safe, fixed-capacity LRU cache with per-entry TTL.
// All operations (Get, Set, Delete) are O(1).
type LRUCache struct {
	mu           sync.Mutex
	items        map[string]*cacheEntry
	evictionList *list.List // front = oldest, back = most recently used
	maxSize      int
}

func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		items:        make(map[string]*cacheEntry),
		evictionList: list.New(),
		maxSize:      maxSize,
	}
}

// Set inserts or updates a key. If the cache is at capacity, the least recently
// used (and non-expired) entry is evicted first.
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry in-place — move to back (most recently used).
	if entry, exists := c.items[key]; exists {
		entry.value = value
		entry.expiresAt = time.Now().Add(ttl)
		c.evictionList.MoveToBack(entry.element)
		return
	}

	// Evict oldest if at capacity.
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	entry.element = c.evictionList.PushBack(entry)
	c.items[key] = entry
}

// Get retrieves a value, refreshing its LRU position. Returns (nil, false) on
// miss or if the entry has expired (it is lazily deleted on access).
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		// Lazy expiry — remove stale entry.
		c.evictionList.Remove(entry.element)
		delete(c.items, key)
		return nil, false
	}

	// Promote to most recently used.
	c.evictionList.MoveToBack(entry.element)
	return entry.value, true
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.items[key]; exists {
		c.evictionList.Remove(entry.element)
		delete(c.items, key)
	}
}

// Clear removes all entries.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheEntry)
	c.evictionList.Init()
}

// Size returns the number of entries currently in the cache (including expired
// ones that have not been lazily cleaned up yet).
func (c *LRUCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// evictOldest removes the front element (least recently used). Must be called
// with c.mu held.
func (c *LRUCache) evictOldest() {
	front := c.evictionList.Front()
	if front == nil {
		return
	}
	entry := front.Value.(*cacheEntry)
	c.evictionList.Remove(front)
	delete(c.items, entry.key)
}
