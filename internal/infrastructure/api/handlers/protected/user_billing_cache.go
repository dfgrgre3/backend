package protected

import (
	"context"
	"encoding/json"
	"sync"
	"time"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"

	"github.com/gin-gonic/gin"
)

type billingSummaryEntry struct {
	data      gin.H
	expiresAt time.Time
}

var (
	billingSummaryL1    sync.Map
	billingSummaryL1TTL = 30 * time.Second
	billingRedisTTL     = 2 * time.Minute
)

const billingSummaryCachePrefix = "billing_summary:"

func checkBillingCaches(c *gin.Context, cacheKey string) bool {
	if val, ok := billingSummaryL1.Load(cacheKey); ok {
		entry := val.(*billingSummaryEntry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, entry.data)
			return true
		}
		billingSummaryL1.Delete(cacheKey)
	}

	if cache.Redis != nil {
		redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Result()
		cancel()
		if err == nil {
			var cachedData gin.H
			if json.Unmarshal([]byte(cachedVal), &cachedData) == nil {
				billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: cachedData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
				api_response.Success(c, cachedData)
				return true
			}
		}
	}
	return false
}

func storeBillingCache(cacheKey string, responseData gin.H) {
	billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: responseData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
	if cache.Redis != nil {
		go func(key string, data gin.H) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if cacheBytes, err := json.Marshal(data); err == nil {
				cache.Redis.Set(ctx, key, cacheBytes, billingRedisTTL)
			}
		}(cacheKey, responseData)
	}
}
