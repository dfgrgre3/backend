package protected

import (
	"context"
	"encoding/json"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func listResources(c *gin.Context, admin bool) {
	params := parseListResourcesParams(c, admin)

	if !admin {
		if tryL1ResourcesCache(c, params.cacheKey) {
			return
		}
		if cache.Redis != nil {
			if tryRedisResourcesCache(c, params.cacheKey) {
				return
			}
		}
	}

	items, total, err := queryResources(params)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch resources")
		return
	}

	if admin {
		sendAdminResourcesResponse(c, items, total, params)
		return
	}

	sendPublicResourcesResponse(c, items, params)
}

func tryL1ResourcesCache(c *gin.Context, cacheKey string) bool {
	if val, ok := l1ResourceCache.Load(cacheKey); ok {
		entry := val.(*l1ResourceEntry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, entry.items)
			return true
		}
		l1ResourceCache.Delete(cacheKey)
	}
	return false
}

func tryRedisResourcesCache(c *gin.Context, cacheKey string) bool {
	redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
	cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Result()
	cancel()
	if err != nil {
		return false
	}
	var cachedItems []gin.H
	if json.Unmarshal([]byte(cachedVal), &cachedItems) != nil {
		return false
	}
	l1ResourceCache.Store(cacheKey, &l1ResourceEntry{
		items:     cachedItems,
		expiresAt: time.Now().Add(l1ResourceTTL),
	})
	api_response.Success(c, cachedItems)
	return true
}

func queryResources(params listResourcesParams) ([]gin.H, int64, error) {
	query := buildResourceQuery(params)

	var total int64
	if params.admin {
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	items, err := fetchAndFormatResources(query, params)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func fetchAndFormatResources(query *gorm.DB, params listResourcesParams) ([]gin.H, error) {
	var resources []models.Resource
	if err := query.Order("created_at DESC").Limit(params.limit).Offset((params.page - 1) * params.limit).Find(&resources).Error; err != nil {
		return nil, err
	}

	items := make([]gin.H, 0, len(resources))
	for _, resource := range resources {
		items = append(items, formatResourceItem(resource, params.admin))
	}
	return items, nil
}

func sendAdminResourcesResponse(c *gin.Context, items []gin.H, total int64, params listResourcesParams) {
	pagination := gin.H{
		"page": params.page, "limit": params.limit, "total": total,
		"totalPages": (total + int64(params.limit) - 1) / int64(params.limit),
	}
	api_response.Success(c, gin.H{
		"success":    true,
		"resources":  items,
		"items":      items,
		"data":       gin.H{"resources": items, "items": items, "pagination": pagination},
		"pagination": pagination,
		"stats": gin.H{
			"total": total,
		},
	})
}

func sendPublicResourcesResponse(c *gin.Context, items []gin.H, params listResourcesParams) {
	if len(items) > 0 {
		l1ResourceCache.Store(params.cacheKey, &l1ResourceEntry{
			items:     items,
			expiresAt: time.Now().Add(l1ResourceTTL),
		})
		if cache.Redis != nil {
			go func(key string, data []gin.H) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if cacheBytes, err := json.Marshal(data); err == nil {
					cache.Redis.Set(ctx, key, cacheBytes, 5*time.Minute)
				}
			}(params.cacheKey, items)
		}
	}
	api_response.Success(c, items)
}

func GetResources(c *gin.Context) {
	listResources(c, false)
}
