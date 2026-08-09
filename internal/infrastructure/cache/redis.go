package cache

import (
	"context"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	redisconn "thanawy-backend/internal/infrastructure/cache/redis"
)

// Compatibility facade over the cache/redis package.
// New code should import cache/redis directly.

// Redis is kept as a package-level variable for backward compatibility with
// existing call sites that reference cache.Redis directly. It mirrors the
// client owned by the cache/redis package and is updated by ConnectRedis /
// CloseRedis. Prefer GetRedis() in new code.
var Redis *goredis.Client

// GetRedis returns the active Redis client in a thread-safe manner.
// Returns nil if Redis is not connected or has been disabled.
func GetRedis() *goredis.Client { return redisconn.Get() }

// IsRedisAvailable returns true if a Redis client is currently connected.
func IsRedisAvailable() bool { return redisconn.IsAvailable() }

// RedisHealthCheck performs a lightweight PING against Redis.
func RedisHealthCheck(ctx context.Context) error { return redisconn.HealthCheck(ctx) }

func ConnectRedis(ctx context.Context, url string) error {
	err := redisconn.Connect(ctx, url)
	Redis = redisconn.Get()
	return err
}

func CloseRedis() error {
	err := redisconn.Close()
	Redis = nil
	return err
}

// ParseAsynqRedisConnOpt returns tuned asynq.RedisConnOpt with safe network
// timeouts and pool size for worker processes under load.
func ParseAsynqRedisConnOpt(redisAddr string) asynq.RedisConnOpt {
	return redisconn.ParseAsynqConnOpt(redisAddr)
}
