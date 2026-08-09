package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"thanawy-backend/internal/infrastructure/workers"
	"time"

	"github.com/gin-gonic/gin"
)

// ─── L1 In-Process Cache ─────────────────────────────────────────────
type notifL1Entry struct {
	data      []byte
	expiresAt time.Time
}

var (
	notificationsL1    sync.Map // Key: string -> *notifL1Entry
	notificationsL1TTL = 15 * time.Second
)

// Cache Key Format Constants
const (
	notifL1CacheKeyFormat = "notif:l1:%s:%d:%s"      // userID:limit:before
	notifRedisCacheKeyFmt = "notifications:%s:%d:%s" // userID:limit:before
	notifUserCachePattern = "notifications:%s:*"     // Redis wildcard pattern for user cache invalidation
)

// GetNotifications handles fetching user notifications with Keyset Pagination & Redis L2 / L1 Caching
func GetNotifications(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userIDStr := userId.(string)
	limit, offset, beforeTime := parseNotificationsPagination(c)
	var notifications []models.Notification

	beforeKey := "latest"
	if beforeTime != nil {
		beforeKey = strconv.FormatInt(beforeTime.UnixNano(), 10)
	}

	l1Key := fmt.Sprintf(notifL1CacheKeyFormat, userIDStr, limit, beforeKey)
	redisKey := fmt.Sprintf(notifRedisCacheKeyFmt, userIDStr, limit, beforeKey)
	useCache := offset == 0

	// ── 1. L1 In-Process Cache Lookup ─────────────────────────────────
	if useCache {
		if tryNotificationsL1Cache(c, l1Key) {
			return
		}
	}

	// ── 2. L2 Redis Cache Lookup ──────────────────────────────────────
	if cache.Redis != nil && useCache {
		if tryNotificationsRedisCache(c, redisKey, l1Key) {
			return
		}
	}

	// ── 3. Database Query (Optimized Index Scan) ─────────────────────
	notifications, err := fetchNotificationsFromDB(c.Request.Context(), userIDStr, limit, offset, beforeTime)
	if err != nil {
		log.Printf("[GetNotifications] DB Error for user %s: %v", userIDStr, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch notifications")
		return
	}

	// ── 4. Asynchronous Cache Warming ────────────────────────────────
	if useCache {
		warmNotificationsCache(redisKey, l1Key, notifications)
	}

	api_response.Success(c, notifications)
}

func parseNotificationsPagination(c *gin.Context) (int, int, *time.Time) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	beforeStr := c.Query("before")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50 // Guard against oversized payload DB queries
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var beforeTime *time.Time
	if beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			beforeTime = &t
		}
	}

	if beforeTime != nil && offset > 0 {
		offset = 0
	}

	return limit, offset, beforeTime
}

func tryNotificationsL1Cache(c *gin.Context, l1Key string) bool {
	if raw, ok := notificationsL1.Load(l1Key); ok {
		entry := raw.(*notifL1Entry)
		if time.Now().Before(entry.expiresAt) {
			var notifications []models.Notification
			if json.Unmarshal(entry.data, &notifications) == nil {
				api_response.Success(c, notifications)
				return true
			}
		}
	}
	return false
}

func tryNotificationsRedisCache(c *gin.Context, redisKey, l1Key string) bool {
	redisCtx, cancel := context.WithTimeout(c.Request.Context(), 150*time.Millisecond)
	defer cancel()

	cachedVal, err := cache.Redis.Get(redisCtx, redisKey).Result()
	if err != nil {
		return false
	}

	var notifications []models.Notification
	if json.Unmarshal([]byte(cachedVal), &notifications) != nil {
		return false
	}

	// Populate L1 cache on L2 hit
	notificationsL1.Store(l1Key, &notifL1Entry{
		data:      []byte(cachedVal),
		expiresAt: time.Now().Add(notificationsL1TTL),
	})

	api_response.Success(c, notifications)
	return true
}

func fetchNotificationsFromDB(ctx context.Context, userID string, limit, offset int, beforeTime *time.Time) ([]models.Notification, error) {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	var notifications []models.Notification

	query := readDB.WithContext(ctx).
		Select("id", "title", "message", "type", "is_read", "created_at", "link", "icon").
		Where("user_id = ?", userID)

	// Keyset/Cursor Pagination Filter
	if beforeTime != nil {
		query = query.Where("created_at < ?", *beforeTime)
	}

	query = query.Order("created_at DESC").Limit(limit)
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

func warmNotificationsCache(redisKey, l1Key string, notifications []models.Notification) {
	cachedData, err := json.Marshal(notifications)
	if err != nil {
		return
	}

	// 1. Populate L1 Cache
	notificationsL1.Store(l1Key, &notifL1Entry{
		data:      cachedData,
		expiresAt: time.Now().Add(notificationsL1TTL),
	})

	// 2. Populate Redis L2 Cache with 30s TTL asynchronously
	if cache.Redis != nil {
		go func(key string, val []byte) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Recovered] Panic in Redis cache write: %v", r)
				}
			}()

			writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cache.Redis.Set(writeCtx, key, val, 30*time.Second)
		}(redisKey, cachedData)
	}
}

// MarkNotificationRead marks specific or all user notifications as read and clears user notification caches
func MarkNotificationRead(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userIDStr := userId.(string)
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		api_response.Error(c, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	if req.ID != "" {
		if err := db.DB.Model(&models.Notification{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.ID, userIDStr).
			Update("is_read", true).Error; err != nil {
			log.Printf("[MarkNotificationRead] DB Error: %v", err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to update notification")
			return
		}
	} else {
		if err := db.DB.Model(&models.Notification{}).
			Where("user_id = ? AND deleted_at IS NULL", userIDStr).
			Update("is_read", true).Error; err != nil {
			log.Printf("[MarkNotificationRead] DB Error: %v", err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to update notifications")
			return
		}
	}

	// Invalidate user notification caches
	InvalidateUserNotificationCache(userIDStr)

	// Broadcast WebSocket event
	refreshMsg, _ := json.Marshal(gin.H{"type": "refresh_notifications"})
	GlobalHub.NotifyUser(userIDStr, refreshMsg)

	api_response.Success(c, gin.H{"success": true})
}

// InvalidateUserNotificationCache clears both L1 and L2 Redis notification caches for a given user
func InvalidateUserNotificationCache(userID string) {
	// 1. Flush matching L1 memory entries
	notificationsL1.Range(func(key, value any) bool {
		kStr, ok := key.(string)
		if ok && len(kStr) > len(userID) && kStr[9:9+len(userID)] == userID {
			notificationsL1.Delete(key)
		}
		return true
	})

	// 2. Flush Redis L2 Cache Keys
	if cache.Redis != nil {
		go func(uID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			pattern := fmt.Sprintf(notifUserCachePattern, uID)
			iter := cache.Redis.Scan(ctx, 0, pattern, 0).Iterator()
			for iter.Next(ctx) {
				cache.Redis.Del(ctx, iter.Val())
			}
			// Also invalidate unread notification count
			cache.Redis.Del(ctx, fmt.Sprintf("unread_notif_count:%s", uID))
		}(userID)
	}
}

// CreateNotificationTask enqueues notification tasks to Asynq worker queue
func CreateNotificationTask(c *gin.Context) {
	var req worker.NotificationPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := worker.EnqueueNotification(req); err != nil {
		log.Printf("[CreateNotificationTask] Enqueue Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to enqueue notification")
		return
	}

	api_response.Success(c, gin.H{"status": "Notification enqueued"})
}
