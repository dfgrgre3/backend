package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Using system env vars")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL not set")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	deleted := 0
	patterns := []string{
		"rate_limit:ip:*",
		"rate_limit:user:*",
		"rate_limit:endpoint:*",
		"login_attempts:*",
	}

	for _, pattern := range patterns {
		deleted += clearPattern(ctx, client, pattern)
	}

	fmt.Printf("✅ Cleared %d rate limit keys from Redis\n", deleted)
	fmt.Println("   You can now log in again.")
}

// clearPattern scans and deletes all keys matching the given pattern from Redis.
// Returns the number of deleted keys.
func clearPattern(ctx context.Context, client *redis.Client, pattern string) int {
	var cursor uint64
	deleted := 0
	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Printf("Scan error for %s: %v", pattern, err)
			break
		}
		if n := deleteKeys(ctx, client, keys, pattern); n > 0 {
			deleted += n
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return deleted
}

// deleteKeys deletes the given keys from Redis and logs the operation.
// Returns the number of deleted keys.
func deleteKeys(ctx context.Context, client *redis.Client, keys []string, pattern string) int {
	if len(keys) == 0 {
		return 0
	}
	if err := client.Del(ctx, keys...).Err(); err != nil {
		log.Printf("Del error: %v", err)
		return 0
	}
	fmt.Printf("   Cleared %d keys matching '%s'\n", len(keys), pattern)
	return len(keys)
}
