package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  In-memory fallback rate limiter (used when Redis is unavailable)
// ─────────────────────────────────────────────

type inMemoryEntry struct {
	count     int
	windowEnd time.Time
}

// InMemoryRateLimiter provides a local, single-instance rate limiter used as a
// fallback when Redis is not connected. This prevents the API from returning
// 503 (fail-closed) during local development when Redis is down, while still
// providing basic per-IP throttling.
type InMemoryRateLimiter struct {
	mu    sync.Mutex
	store map[string]*inMemoryEntry
}

var (
	globalInMemoryLimiter = &InMemoryRateLimiter{store: make(map[string]*inMemoryEntry)}
	inMemoryMu            sync.Mutex
	cleanupStarted        sync.Once
)

const (
	rateLimiterCleanupInterval = 5 * time.Minute
	rateLimiterMaxEntries      = 10000
)

// startRateLimiterCleanup periodically removes expired entries to prevent memory leaks.
func startRateLimiterCleanup(rl *InMemoryRateLimiter) {
	cleanupStarted.Do(func() {
		go func() {
			ticker := time.NewTicker(rateLimiterCleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				rl.cleanup()
			}
		}()
	})
}

// cleanup removes expired entries and enforces a maximum size to prevent memory leaks.
func (rl *InMemoryRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.store {
		if now.After(entry.windowEnd) {
			delete(rl.store, key)
		}
	}

	// If still too many entries after cleanup, enforce a hard limit
	if len(rl.store) > rateLimiterMaxEntries {
		// Clear half the entries (random eviction is fine for rate limiter)
		count := 0
		for key := range rl.store {
			delete(rl.store, key)
			count++
			if count >= rateLimiterMaxEntries/2 {
				break
			}
		}
	}
}

// getInMemoryLimiter returns the shared in-memory limiter singleton.
func getInMemoryLimiter() *InMemoryRateLimiter {
	inMemoryMu.Lock()
	defer inMemoryMu.Unlock()
	if globalInMemoryLimiter == nil {
		globalInMemoryLimiter = &InMemoryRateLimiter{store: make(map[string]*inMemoryEntry)}
	}
	startRateLimiterCleanup(globalInMemoryLimiter)
	return globalInMemoryLimiter
}

// increment records a request for key and returns the count within the window.
// Expired entries are lazily cleaned up on access.
func (rl *InMemoryRateLimiter) increment(key string, window time.Duration) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.store[key]
	if !ok || now.After(entry.windowEnd) {
		// Start a new window.
		rl.store[key] = &inMemoryEntry{count: 1, windowEnd: now.Add(window)}
		return 1
	}

	entry.count++
	return entry.count
}

// RateLimitByIPInMemory enforces a per-IP limit using the in-memory store.
func RateLimitByIPInMemory(limit int, window time.Duration) gin.HandlerFunc {
	rl := getInMemoryLimiter()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:ip:%s", ip)

		count := rl.increment(key, window)

		// Set rate limit headers
		c.Header(headerRateLimitLimit, fmt.Sprintf("%d", limit))
		c.Header(headerRateLimitRemaining, fmt.Sprintf("%d", max(0, limit-count)))
		c.Header(headerRateLimitReset, fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		if count > limit {
			c.Header(headerRetryAfter, fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUserInMemory enforces a per-user limit using the in-memory store.
func RateLimitByUserInMemory(limit int, window time.Duration) gin.HandlerFunc {
	rl := getInMemoryLimiter()
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		key := fmt.Sprintf("rate_limit:user:%s", userID)
		count := rl.increment(key, window)

		c.Header(headerRateLimitLimit, fmt.Sprintf("%d", limit))
		c.Header(headerRateLimitRemaining, fmt.Sprintf("%d", max(0, limit-count)))
		c.Header(headerRateLimitReset, fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		if count > limit {
			c.Header(headerRetryAfter, fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "user rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
