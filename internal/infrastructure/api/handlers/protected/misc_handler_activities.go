package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

// ─── in-process L1 cache for recent-activities ────────────────────────────────
// Keeps a small in-memory snapshot so that short-burst repeated requests
// (e.g. React StrictMode double-render, rapid page navigations) never hit
// the remote Redis or the database at all.
var (
	activitiesL1    = cache.NewLRUCache(1000)
	activitiesL1TTL = 20 * time.Second // same tenant sees fresh data within 20 s
)

type recentActivitiesParams struct {
	limit  int
	offset int
	useL1  bool
	l1Key  string
}

func parseRecentActivitiesParams(c *gin.Context) recentActivitiesParams {
	limit := 10
	offset := 0
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && v > 0 {
		if v > 20 {
			v = 20
		}
		limit = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && v >= 0 {
		offset = v
	}
	useL1 := offset == 0 && limit <= 10
	l1Key := fmt.Sprintf("ra:%s:%d", c.GetString("userId"), limit)
	return recentActivitiesParams{limit: limit, offset: offset, useL1: useL1, l1Key: l1Key}
}

func GetRecentActivities(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	params := parseRecentActivitiesParams(c)
	params.l1Key = fmt.Sprintf("ra:%s:%d", userId, params.limit)

	if params.useL1 {
		if tryActivitiesL1Cache(c, params.l1Key) {
			return
		}
	}

	redisKey := fmt.Sprintf("recent_activities:%s:%d", userId, params.limit)
	if cache.Redis != nil && params.offset == 0 {
		if tryActivitiesRedisCache(c, redisKey, params) {
			return
		}
	}

	notifications := fetchRecentActivities(c, userId.(string), params)
	if notifications == nil {
		return
	}

	warmActivitiesCache(redisKey, params, notifications)
	api_response.Success(c, gin.H{"activities": buildActivitiesResponse(notifications)})
}

func tryActivitiesL1Cache(c *gin.Context, l1Key string) bool {
	if val, ok := activitiesL1.Get(l1Key); ok {
		var notifications []models.Notification
		if json.Unmarshal(val.([]byte), &notifications) == nil {
			api_response.Success(c, gin.H{"activities": buildActivitiesResponse(notifications)})
			return true
		}
	}
	return false
}

func tryActivitiesRedisCache(c *gin.Context, redisKey string, params recentActivitiesParams) bool {
	redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
	cachedVal, err := cache.Redis.Get(redisCtx, redisKey).Result()
	cancel()
	if err != nil {
		return false
	}
	var notifications []models.Notification
	if json.Unmarshal([]byte(cachedVal), &notifications) != nil {
		return false
	}
	if params.useL1 {
		activitiesL1.Set(params.l1Key, []byte(cachedVal), activitiesL1TTL)
	}
	api_response.Success(c, gin.H{"activities": buildActivitiesResponse(notifications)})
	return true
}

func fetchRecentActivities(c *gin.Context, userId string, params recentActivitiesParams) []models.Notification {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	var notifications []models.Notification
	if err := readDB.
		Select("id", "title", "message", "type", "is_read", "created_at").
		Where("user_id = ?", userId).
		Order("created_at desc").
		Limit(params.limit).
		Offset(params.offset).
		Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch activities")
		return nil
	}
	return notifications
}

func warmActivitiesCache(redisKey string, params recentActivitiesParams, notifications []models.Notification) {
	if params.offset > 0 || len(notifications) == 0 {
		return
	}
	cachedData, err := json.Marshal(notifications)
	if err != nil {
		return
	}
	if cache.Redis != nil {
		go func(redisKey string, cachedData []byte) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Recovered] panic in recent activities Redis write: %v", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Redis.Set(ctx, redisKey, cachedData, 60*time.Second)
		}(redisKey, cachedData)
	}
	if params.useL1 {
		activitiesL1.Set(params.l1Key, cachedData, activitiesL1TTL)
	}
}

// buildActivitiesResponse converts notifications to activity format
func buildActivitiesResponse(notifications []models.Notification) []gin.H {
	activities := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		activities = append(activities, gin.H{
			"id":          n.ID,
			"type":        strings.ToLower(string(n.Type)),
			"title":       n.Title,
			"description": n.Message,
			"timestamp":   n.CreatedAt,
			"read":        n.IsRead,
		})
	}
	return activities
}
