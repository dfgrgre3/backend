package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

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
