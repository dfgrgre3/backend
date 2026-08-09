package redisconn

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
	client   *redis.Client
	clientMu sync.RWMutex
)

// Get returns the active Redis client in a thread-safe manner.
// Returns nil if Redis is not connected or has been disabled.
func Get() *redis.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client
}

// IsAvailable returns true if a Redis client is currently connected.
func IsAvailable() bool {
	return Get() != nil
}

// HealthCheck performs a lightweight PING against Redis and returns an
// error if Redis is unavailable or the ping fails.
func HealthCheck(ctx context.Context) error {
	c := Get()
	if c == nil {
		return fmt.Errorf("redis not connected")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return c.Ping(pingCtx).Err()
}

func Connect(ctx context.Context, url string) error {
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
	poolSize, minIdleConns := poolDefaults()
	opts.PoolSize = envInt("REDIS_POOL_SIZE", poolSize)
	opts.MinIdleConns = envInt("REDIS_MIN_IDLE_CONNS", minIdleConns)
	opts.MaxRetries = envInt("REDIS_MAX_RETRIES", 5)
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 5 * time.Second
	opts.WriteTimeout = 5 * time.Second
	opts.PoolTimeout = 10 * time.Second
	opts.ConnMaxLifetime = 30 * time.Minute
	// Release idle connections after 10 minutes to avoid holding connections
	// that the Redis server may have already closed server-side.
	opts.ConnMaxIdleTime = 10 * time.Minute

	c := redis.NewClient(opts)
	attempts := envInt("REDIS_CONNECT_ATTEMPTS", 5)
	delay := time.Duration(envInt("REDIS_CONNECT_RETRY_SECONDS", 2)) * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
		lastErr = c.Ping(pingCtx).Err()
		cancel()
		if lastErr == nil {
			clientMu.Lock()
			client = c
			clientMu.Unlock()
			// Sanitize URL for logging - don't expose credentials
			safeURL := sanitizeURL(url)
			tlsInfo := ""
			if opts.TLSConfig != nil || strings.HasPrefix(url, "rediss://") {
				tlsInfo = " (TLS enabled)"
			}
			if attempt > 1 {
				log.Printf("Redis reconnection successful%s (attempt %d, PoolSize=%d, MinIdleConns=%d) [source: %s]",
					tlsInfo, attempt, opts.PoolSize, opts.MinIdleConns, safeURL)
			} else {
				log.Printf("Redis connection established%s (PoolSize=%d, MinIdleConns=%d) [source: %s]",
					tlsInfo, opts.PoolSize, opts.MinIdleConns, safeURL)
			}
			checkVersion(ctx, c)
			return nil
		}
		if attempt < attempts {
			log.Printf("Redis connection attempt %d/%d failed: %v. Retrying in %s...", attempt, attempts, lastErr, delay)
			select {
			case <-ctx.Done():
				_ = c.Close()
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	_ = c.Close()
	return fmt.Errorf("connect to Redis after %d attempts: %w", attempts, lastErr)
}

func Close() error {
	clientMu.Lock()
	c := client
	client = nil
	clientMu.Unlock()

	if c == nil {
		return nil
	}
	log.Println("Closing Redis connection pool...")
	return c.Close()
}

func poolDefaults() (poolSize, minIdleConns int) {
	if isServerlessEnv() {
		return 5, 1
	}
	return 20, 2
}

func envInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func checkVersion(ctx context.Context, c *redis.Client) {
	info, err := c.Info(ctx, "server").Result()
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
			// Only log at debug level to avoid exposing version in production logs
			log.Printf("Redis server version: %s", versionStr)
			return
		}
	}
}

// isServerlessEnv detects Vercel, AWS Lambda, and similar ephemeral function environments.
func isServerlessEnv() bool {
	return os.Getenv("VERCEL") == "1" ||
		os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" ||
		os.Getenv("SERVERLESS") == "1"
}

// sanitizeURL removes credentials from Redis URL for safe logging
func sanitizeURL(url string) string {
	if url == "" {
		return "<not set>"
	}
	// If URL contains @, it likely has credentials embedded
	if strings.Contains(url, "@") {
		// Try to parse and reconstruct safely
		if u, err := redis.ParseURL(url); err == nil {
			return fmt.Sprintf("redis://%s/%d", u.Addr, u.DB)
		}
	}
	return url
}
