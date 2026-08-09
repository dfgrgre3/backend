package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	rateLimiterUnavailable   = "rate limiter unavailable, please try again later"
	headerRateLimitLimit     = "X-RateLimit-Limit"
	headerRateLimitRemaining = "X-RateLimit-Remaining"
	headerRateLimitReset     = "X-RateLimit-Reset"
	headerRetryAfter         = "Retry-After"
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

// ─────────────────────────────────────────────
//  Redis-backed rate limiter
// ─────────────────────────────────────────────

// RateLimiter holds Redis client for distributed rate limiting
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{client: redisClient}
}

// RateLimitByIP creates a rate limiting middleware by IP address
// limit: requests per window
// window: time window duration
func (rl *RateLimiter) RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:ip:%s", ip)

		count, err := rl.incrementCounter(c.Request.Context(), key, window)
		if err != nil {
			// If Redis fails, fall back to the in-memory limiter so the API
			// remains available (fail open with local throttling).
			RateLimitByIPInMemory(limit, window)(c)
			return
		}

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

// RateLimitByUser creates a rate limiting middleware by authenticated user ID
// limit: requests per window
// window: time window duration
func (rl *RateLimiter) RateLimitByUser(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			// Not authenticated, skip user-level rate limiting
			c.Next()
			return
		}

		key := fmt.Sprintf("rate_limit:user:%s", userID)
		count, err := rl.incrementCounter(c.Request.Context(), key, window)
		if err != nil {
			RateLimitByUserInMemory(limit, window)(c)
			return
		}

		// Set rate limit headers
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

// RateLimitByEndpoint creates per-endpoint rate limiting
// endpoint: unique identifier for the endpoint
// limit: requests per window
// window: time window duration
func (rl *RateLimiter) RateLimitByEndpoint(endpoint string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If user is authenticated, include user ID for stricter per-user limits
		if userID, exists := c.Get("user_id"); exists {
			endpoint = fmt.Sprintf("%s:user:%s", endpoint, userID)
		}

		key := fmt.Sprintf("rate_limit:endpoint:%s", endpoint)

		count, err := rl.incrementCounter(c.Request.Context(), key, window)
		if err != nil {
			// Fallback: use the same in-memory limiter keyed by endpoint.
			rlMem := getInMemoryLimiter()
			count = rlMem.increment(key, window)
		}

		c.Header(headerRateLimitLimit, fmt.Sprintf("%d", limit))
		c.Header(headerRateLimitRemaining, fmt.Sprintf("%d", max(0, limit-count)))
		c.Header(headerRateLimitReset, fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		if count > limit {
			c.Header(headerRetryAfter, fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "endpoint rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// incrementCounter increments a counter in Redis and returns the new count
// Uses INCR + EXPIRE for atomicity
func (rl *RateLimiter) incrementCounter(ctx context.Context, key string, window time.Duration) (int, error) {
	// Fail closed if Redis client is nil
	if rl.client == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	// INCR is atomic
	val, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Set expiration on first request (TTL = window)
	if val == 1 {
		rl.client.Expire(ctx, key, window)
	}

	return int(val), nil
}

// SlidingWindowRateLimit uses sliding window algorithm (more accurate)
// Useful for APIs that need precise rate limiting
func (rl *RateLimiter) SlidingWindowRateLimit(key string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.client == nil {
			// Fall back to in-memory if Redis is unavailable.
			RateLimitByIPInMemory(limit, window)(c)
			return
		}

		ctx := c.Request.Context()
		now := time.Now().UnixMilli()
		windowStart := now - window.Milliseconds()

		// Remove old entries outside the window
		rl.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart))

		// Count requests in current window
		count, err := rl.client.ZCard(ctx, key).Result()
		if err != nil {
			RateLimitByIPInMemory(limit, window)(c)
			return
		}

		if count >= int64(limit) {
			c.Header(headerRetryAfter, fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		// Add current request
		rl.client.ZAdd(ctx, key, redis.Z{
			Score:  float64(now),
			Member: fmt.Sprintf("%d", now),
		})
		rl.client.Expire(ctx, key, window)

		c.Next()
	}
}

// ─────────────────────────────────────────────
//  Convenience rate limiters (with in-memory fallback)
// ─────────────────────────────────────────────

// LoginRateLimiter provides rate limiting for login attempts.
// If Redis is unavailable, falls back to the in-memory limiter so users can
// still log in during local development or a Redis outage.
func LoginRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache.Redis == nil {
			RateLimitByIPInMemory(20, time.Minute)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(20, time.Minute)(c)
	}
}

// AuthRateLimiter provides rate limiting for authentication-related requests.
// Falls back to the in-memory limiter when Redis is unavailable.
func AuthRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache.Redis == nil {
			RateLimitByIPInMemory(60, time.Minute)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(60, time.Minute)(c)
	}
}

// RefreshTokenRateLimiter provides rate limiting specifically for token refresh endpoints.
// Uses a more generous limit (30/min) to prevent abuse while allowing normal refresh flows.
// Falls back to the in-memory limiter when Redis is unavailable.
func RefreshTokenRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache.Redis == nil {
			RateLimitByIPInMemory(30, time.Minute)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(30, time.Minute)(c)
	}
}

// GlobalRateLimiter provides rate limiting for all API requests.
// Falls back to the in-memory limiter when Redis is unavailable.
func GlobalRateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache.Redis == nil {
			RateLimitByIPInMemory(limit, window)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(limit, window)(c)
	}
}

// AIRateLimiter provides rate limiting for AI requests.
// Falls back to the in-memory limiter when Redis is unavailable.
// The limit is intentionally generous (30/min) because interactive AI chat
// sessions involve rapid back-and-forth messages, and the frontend may also
// poll for async job status (exam generation, summarization, essay grading).
func AIRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("userId")
		if exists && userIDValue != nil {
			if userIDStr, ok := userIDValue.(string); ok && userIDStr != "" {
				c.Set("user_id", userIDStr)
				if cache.Redis == nil {
					RateLimitByUserInMemory(30, time.Minute)(c)
					return
				}
				NewRateLimiter(cache.Redis).RateLimitByUser(30, time.Minute)(c)
				return
			}
		}

		if cache.Redis == nil {
			RateLimitByIPInMemory(30, time.Minute)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(30, time.Minute)(c)
	}
}

// WebSocketRateLimiter provides rate limiting for WebSocket connections.
// Falls back to the in-memory limiter when Redis is unavailable.
func WebSocketRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("userId")
		if exists && userIDValue != nil {
			if userIDStr, ok := userIDValue.(string); ok && userIDStr != "" {
				c.Set("user_id", userIDStr)
				if cache.Redis == nil {
					RateLimitByUserInMemory(5, time.Minute)(c)
					return
				}
				NewRateLimiter(cache.Redis).RateLimitByUser(5, time.Minute)(c)
				return
			}
		}

		if cache.Redis == nil {
			RateLimitByIPInMemory(5, time.Minute)(c)
			return
		}
		NewRateLimiter(cache.Redis).RateLimitByIP(5, time.Minute)(c)
	}
}
