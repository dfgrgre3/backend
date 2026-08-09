package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// dashboardRefreshScopes يربط كل نطاق بمفاتيح الكاش التي يبطلها.
// النطاق "all" يُوسَّع إلى بقية النطاقات عند التنفيذ.
var dashboardRefreshScopes = map[string][]string{
	"kpis":         {"admin:dashboard:stats", "admin:dashboard:stats:*", "admin:dashboard:summary:*"},
	"pending":      {"admin:dashboard:pending:*"},
	"alerts":       {"admin:dashboard:alerts:*"},
	"activities":   {"admin:dashboard:activities:*"},
	"topCourses":   {"admin:dashboard:top-courses:*"},
	"systemHealth": {"admin:dashboard:system-health:*"},
}

// dashboardRefreshCooldown يمنع إعادة التجميع المتكررة التي ترهق قاعدة البيانات.
const dashboardRefreshCooldown = 30 * time.Second

// RefreshDashboardData يُبطل الكاش ليُعاد تجميع البيانات عند الطلب التالي.
//
// POST /api/admin/dashboard/refresh
func RefreshDashboardData(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardRefreshCache) {
		return
	}
	callerID, authenticated := getAuthenticatedUserID(c)
	if !authenticated {
		return
	}

	var body struct {
		Scope    string `json:"scope"`
		TenantID string `json:"tenantId"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	scope := strings.TrimSpace(body.Scope)
	if scope == "" {
		scope = "all"
	}
	if scope != "all" {
		if _, known := dashboardRefreshScopes[scope]; !known {
			api_response.Error(c, http.StatusBadRequest, fmt.Sprintf(
				"scope must be one of all, %s", strings.Join(sortedKeys(mapKeysAsValues(dashboardRefreshScopes)), ", ")))
			return
		}
	}

	ctx := c.Request.Context()

	// قفل قصير العمر لكل مستخدم/نطاق: يمنع الضغط المتكرر على زر التحديث.
	if cache.Redis != nil {
		lockKey := fmt.Sprintf("admin:dashboard:refresh:lock:%s:%s", callerID, scope)
		acquired, err := cache.Redis.SetNX(ctx, lockKey, "1", dashboardRefreshCooldown).Result()
		if err == nil && !acquired {
			api_response.Error(c, http.StatusConflict, "A refresh for this scope is already in progress")
			return
		}
	}

	patterns := make([]string, 0, 8)
	if scope == "all" {
		for _, keys := range dashboardRefreshScopes {
			patterns = append(patterns, keys...)
		}
	} else {
		patterns = append(patterns, dashboardRefreshScopes[scope]...)
	}

	deleted := invalidateDashboardCache(ctx, patterns)

	// Invalidation is synchronous and complete by the time we reply, so the
	// response describes exactly that. It previously returned a random `jobId`
	// that was never persisted and a hardcoded `status`, implying a background
	// job that does not exist and cannot be polled.
	api_response.Success(c, gin.H{
		"scope":           scope,
		"invalidatedKeys": deleted,
		"refreshedAt":     time.Now(),
	})
}

// invalidateDashboardCache يحذف المفاتيح المطابقة. يستخدم SCAN بدل KEYS
// لتجنب حجب Redis على قواعد البيانات الكبيرة.
func invalidateDashboardCache(ctx context.Context, patterns []string) int {
	if cache.Redis == nil {
		return 0
	}
	deleted := 0
	for _, pattern := range patterns {
		if !strings.Contains(pattern, "*") {
			if err := cache.Redis.Del(ctx, pattern).Err(); err == nil {
				deleted++
			}
			continue
		}
		var cursor uint64
		for {
			keys, next, err := cache.Redis.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				if err := cache.Redis.Del(ctx, keys...).Err(); err == nil {
					deleted += len(keys)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	return deleted
}

// mapKeysAsValues يحوّل المفاتيح إلى خريطة نصية لإعادة استخدام sortedKeys.
func mapKeysAsValues(source map[string][]string) map[string]string {
	out := make(map[string]string, len(source))
	for key := range source {
		out[key] = key
	}
	return out
}
