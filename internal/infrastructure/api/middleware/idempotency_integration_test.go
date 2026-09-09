package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"thanawy-backend/internal/infrastructure/cache"
)

// This test intentionally uses a real Redis connection. It validates the
// contract that protects payment/enrollment-style mutations; without Redis,
// the middleware is deliberately fail-closed and this test is not meaningful.
func TestIdempotency_ReplaysCachedMutation(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://:devpassword@localhost:6379"
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Skipf("Redis is not available: %v", err)
	}

	previousRedis := cache.Redis
	cache.Redis = client
	t.Cleanup(func() { cache.Redis = previousRedis })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Idempotency())
	var mutations atomic.Int32
	router.POST("/api/v1/courses/:id/enroll", func(c *gin.Context) {
		mutations.Add(1)
		c.JSON(http.StatusCreated, gin.H{"enrolled": true})
	})

	key := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courses/course-1/enroll", nil)
		req.Header.Set("Idempotency-Key", key)
		router.ServeHTTP(recorder, req)
		return recorder
	}

	first := request()
	second := request()

	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("expected two successful responses, got %d and %d", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replayed response differs: first=%q second=%q", first.Body.String(), second.Body.String())
	}
	if got := mutations.Load(); got != 1 {
		t.Fatalf("expected exactly one mutation, got %d", got)
	}
}
