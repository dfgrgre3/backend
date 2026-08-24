package protected

import (
	"context"
	"fmt"
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

// ─── in-process L1 cache for unread notifications count ──────────
var (
	unreadCountL1    = cache.NewLRUCache(1000)
	unreadCountL1TTL = 20 * time.Second
)

func GetUnreadNotificationsCount(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	count, ok := tryUnreadNotificationsCaches(c, userId)
	if ok {
		api_response.Success(c, gin.H{"count": count})
		return
	}

	count = fetchAndCacheUnreadCount(c, userId.(string))
	api_response.Success(c, gin.H{"count": count})
}

// tryUnreadNotificationsCaches attempts to serve the count from L1 or L2 cache.
// Returns (count, true) if cache was hit, (0, false) otherwise.
func tryUnreadNotificationsCaches(c *gin.Context, userId interface{}) (int64, bool) {
	l1Key := fmt.Sprintf("unc:%s", userId)
	if val, ok := unreadCountL1.Get(l1Key); ok {
		return val.(int64), true
	}

	if cache.Redis != nil {
		cacheKey := fmt.Sprintf("unread_notif_count:%s", userId)
		redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Int()
		cancel()
		if err == nil {
			count := int64(cachedVal)
			unreadCountL1.Set(l1Key, count, unreadCountL1TTL)
			return count, true
		}
	}

	return 0, false
}

func fetchAndCacheUnreadCount(c *gin.Context, userId string) int64 {
	var count int64
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	if err := readDB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userId, false).
		Count(&count).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to count notifications")
		return 0
	}

	// Populate both caches
	if cache.Redis != nil {
		go func(userId string, count int64) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Recovered] panic in unread notifications Redis write: %v", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Redis.Set(ctx, fmt.Sprintf("unread_notif_count:%s", userId), count, time.Minute)
		}(userId, count)
	}
	l1Key := fmt.Sprintf("unc:%s", userId)
	unreadCountL1.Set(l1Key, count, unreadCountL1TTL)

	return count
}
