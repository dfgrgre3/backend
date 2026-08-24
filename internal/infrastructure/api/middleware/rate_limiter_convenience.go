package middleware

import (
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/gin-gonic/gin"
)

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
