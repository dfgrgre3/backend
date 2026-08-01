package db

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	Redis   *redis.Client
	redisMu sync.RWMutex
)

// GetRedis returns the active Redis client in a thread-safe manner.
// Returns nil if Redis is not connected or has been disabled.
// All application code should prefer this over accessing db.Redis directly.
func GetRedis() *redis.Client {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return Redis
}

// IsRedisAvailable returns true if a Redis client is currently connected.
func IsRedisAvailable() bool {
	return GetRedis() != nil
}

// RedisHealthCheck performs a lightweight PING against Redis and returns an
// error if Redis is unavailable or the ping fails.
func RedisHealthCheck(ctx context.Context) error {
	client := GetRedis()
	if client == nil {
		return fmt.Errorf("redis not connected")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.Ping(pingCtx).Err()
}

func ConnectRedis(ctx context.Context, url string) error {
	if url == "" || os.Getenv("DISABLE_REDIS") == "true" {
		log.Println("Redis is disabled via DISABLE_REDIS or empty URL")
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse Redis URL: %w", err)
	}

	// Force TLS if REDIS_TLS environment variable is set to true
	if os.Getenv("REDIS_TLS") == "true" && opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	// Configure Redis connection pooling for massive scale. Defaults are safe for
	// a few app replicas and can be tuned per deployment without code changes.
	poolSize, minIdleConns := redisPoolDefaults()
	opts.PoolSize        = getRedisInt("REDIS_POOL_SIZE", poolSize)
	opts.MinIdleConns    = getRedisInt("REDIS_MIN_IDLE_CONNS", minIdleConns)
	opts.MaxRetries      = getRedisInt("REDIS_MAX_RETRIES", 5)
	opts.DialTimeout     = 5 * time.Second
	opts.ReadTimeout     = 2 * time.Second
	opts.WriteTimeout    = 2 * time.Second
	opts.PoolTimeout     = 4 * time.Second
	opts.ConnMaxLifetime = 30 * time.Minute
	// Release idle connections after 10 minutes to avoid holding connections
	// that the Redis server may have already closed server-side.
	opts.ConnMaxIdleTime = 10 * time.Minute

	client := redis.NewClient(opts)
	attempts := getRedisInt("REDIS_CONNECT_ATTEMPTS", 5)
	delay := time.Duration(getRedisInt("REDIS_CONNECT_RETRY_SECONDS", 2)) * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
		lastErr = client.Ping(pingCtx).Err()
		cancel()
		if lastErr == nil {
			redisMu.Lock()
			Redis = client
			redisMu.Unlock()
			log.Printf("Redis connection established (PoolSize=%d, MinIdleConns=%d)", opts.PoolSize, opts.MinIdleConns)
			checkRedisVersion(ctx, client)
			return nil
		}
		if attempt < attempts {
			log.Printf("Redis is not ready (attempt %d/%d): %v. Retrying in %s...", attempt, attempts, lastErr, delay)
			select {
			case <-ctx.Done():
				_ = client.Close()
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	_ = client.Close()
	return fmt.Errorf("connect to Redis after %d attempts: %w", attempts, lastErr)
}

func CloseRedis() error {
	redisMu.Lock()
	client := Redis
	Redis = nil
	redisMu.Unlock()

	if client == nil {
		return nil
	}
	log.Println("Closing Redis connection pool...")
	return client.Close()
}

func redisPoolDefaults() (poolSize, minIdleConns int) {
	if isServerlessEnv() {
		return 5, 1
	}
	return 20, 2
}

func getRedisInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func checkRedisVersion(ctx context.Context, client *redis.Client) {
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return
	}
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "redis_version:") {
			versionStr := strings.TrimPrefix(line, "redis_version:")
			parts := strings.Split(versionStr, ".")
			if len(parts) > 0 {
				if major, err := strconv.Atoi(parts[0]); err == nil && major < 5 {
					log.Printf("[WARNING] Redis version %s detected (< 5.0). Redis Streams (XADD/XREADGROUP) are NOT supported! Please upgrade Redis to 5.0+ or 7.x.", versionStr)
					return
				}
			}
			log.Printf("Redis server version: %s", versionStr)
			return
		}
	}
}
