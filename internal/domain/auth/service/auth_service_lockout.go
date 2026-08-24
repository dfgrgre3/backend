package authservice

import (
	"context"
	"fmt"
	"thanawy-backend/internal/infrastructure/cache"
	"time"
)

// Lockout Policy Configuration
//
// SECURITY: this entire policy is backed by Redis (see checkAccountLockout /
// recordFailedAttempt below) and fails OPEN — with cache.Redis == nil, login
// attempts are never counted and accounts can never be locked out, with no
// warning at request time. Redis is therefore a hard security dependency in
// production, not just a performance cache; deploying without it silently
// disables brute-force protection on the login endpoint.
const (
	maxFailedAttempts    = 5                // عدد المحاولات الفاشلة المسموحة
	lockoutDuration      = 15 * time.Minute // مدة قفل الحساب (15 دقيقة)
	failedAttemptsPrefix = "failed_attempts:"
	lockoutPrefix        = "lockout:"
)

// getFailedAttemptsKey returns Redis key for counting failed login attempts
func getFailedAttemptsKey(userID string) string {
	return fmt.Sprintf("%s%s", failedAttemptsPrefix, userID)
}

// getLockoutKey returns Redis key for account lockout
func getLockoutKey(userID string) string {
	return fmt.Sprintf("%s%s", lockoutPrefix, userID)
}

// checkAccountLockout checks if the account is currently locked out
// Returns nil if not locked, or an error with remaining time message
func checkAccountLockout(ctx context.Context, userID string) error {
	if cache.Redis == nil {
		// Fails OPEN: without Redis there is no lockout store to check, so
		// every login is treated as "not locked out" regardless of prior
		// failed attempts. See the SECURITY note on the Lockout Policy
		// Configuration block above — this is a deployment requirement, not
		// a caller-visible error, so it stays silent here by design.
		return nil
	}

	lockoutKey := getLockoutKey(userID)
	ttl, err := cache.Redis.TTL(ctx, lockoutKey).Result()
	if err != nil || ttl <= 0 {
		return nil // Not locked out
	}

	minutesRemaining := int(ttl.Minutes()) + 1
	return fmt.Errorf("account locked due to too many failed attempts. Try again in %d minute(s)", minutesRemaining)
}

// recordFailedAttempt increments the failed attempt counter.
// Locks the account if threshold is reached.
func recordFailedAttempt(ctx context.Context, userID string) {
	if cache.Redis == nil || userID == "" {
		return
	}

	failedKey := getFailedAttemptsKey(userID)
	lockoutKey := getLockoutKey(userID)

	// Increment failed attempts counter
	count, err := cache.Redis.Incr(ctx, failedKey).Result()
	if err != nil {
		return
	}

	// Set expiry on first attempt to track over rolling window
	if count == 1 {
		cache.Redis.Expire(ctx, failedKey, lockoutDuration)
	}

	// Lock account if threshold is reached
	if count >= maxFailedAttempts {
		cache.Redis.Set(ctx, lockoutKey, "locked", lockoutDuration)
		cache.Redis.Del(ctx, failedKey) // Reset failed counter after locking
	}
}

// resetFailedAttempts clears the failed attempt counter on successful login
func resetFailedAttempts(ctx context.Context, userID string) {
	if cache.Redis == nil || userID == "" {
		return
	}
	cache.Redis.Del(ctx, getFailedAttemptsKey(userID))
	cache.Redis.Del(ctx, getLockoutKey(userID))
}
