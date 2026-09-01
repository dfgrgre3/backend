package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "devpassword",
		DB:       0,
	})
	ctx := context.Background()

	// Find and delete all rate limit keys for login
	keys, err := client.Keys(ctx, "rate_limit:ip:*").Result()
	if err != nil {
		fmt.Printf("Error finding keys: %v\n", err)
		return
	}

	fmt.Printf("Found %d rate limit keys:\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  - %s\n", k)
	}

	if len(keys) > 0 {
		deleted, err := client.Del(ctx, keys...).Result()
		if err != nil {
			fmt.Printf("Error deleting keys: %v\n", err)
			return
		}
		fmt.Printf("Deleted %d keys successfully!\n", deleted)
	} else {
		fmt.Println("No rate limit keys found in Redis.")
	}
}
