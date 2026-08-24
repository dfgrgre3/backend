package middleware

const (
	rateLimiterUnavailable   = "rate limiter unavailable, please try again later"
	headerRateLimitLimit     = "X-RateLimit-Limit"
	headerRateLimitRemaining = "X-RateLimit-Remaining"
	headerRateLimitReset     = "X-RateLimit-Reset"
	headerRetryAfter         = "Retry-After"
)

// Rate limiting is split across several files in this package (all sharing
// package middleware): this file (shared headers/messages),
// rate_limiter_memory.go (in-memory fallback limiter),
// rate_limiter_redis.go (Redis-backed RateLimiter) and
// rate_limiter_convenience.go (pre-configured wrapper functions).
