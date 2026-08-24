package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

// dashboardActivityDays is the size of the activity window returned to the
// admin UI. The heatmap widget renders 12 weeks, so the backend must return
// the same range instead of only the last 7 days.
const dashboardActivityDays = 84

const dashboardCacheTTL = 5 * time.Minute

var adminDashboardSF singleflight.Group

// GetAdminDashboard serves the aggregated admin dashboard payload, backed by a
// Redis cache and singleflight so concurrent misses only compute it once.
func GetAdminDashboard(c *gin.Context) {
	timeParam := c.Query("time")
	cacheKey := "admin:dashboard:stats"
	if timeParam != "" {
		cacheKey = fmt.Sprintf("admin:dashboard:stats:%s", timeParam)
	}

	if payload, ok := getCachedAdminDashboard(c.Request.Context(), cacheKey); ok {
		api_response.Success(c, filterDashboardPayload(c, payload))
		return
	}

	// This closure may serve multiple concurrent callers via singleflight, so
	// it intentionally uses context.Background() rather than this request's
	// context: cancelling it because *this* caller disconnected would also
	// abort the response every other waiting request needs.
	payload, err, _ := adminDashboardSF.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := getCachedAdminDashboard(context.Background(), cacheKey); ok {
			return cached, nil
		}
		return buildAdminDashboardPayload(cacheKey, timeParam)
	})
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}
	api_response.Success(c, filterDashboardPayload(c, payload))
}

func filterDashboardPayload(c *gin.Context, rawPayload interface{}) interface{} {
	payload, ok := rawPayload.(map[string]interface{})
	if !ok {
		return rawPayload
	}

	// Create a shallow copy of the top-level map
	filtered := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		filtered[k] = v
	}

	// 1. Gating stats map
	if statsVal, ok := filtered["stats"]; ok {
		if statsMap, ok := statsVal.(map[string]interface{}); ok {
			statsCopy := make(map[string]interface{}, len(statsMap))
			for sk, sv := range statsMap {
				statsCopy[sk] = sv
			}
			// If not allowed to see KPIs
			if !dashboardCan(c, models.PermDashboardViewKPIs) {
				kpisToHide := []string{"totalUsers", "activeStudents", "totalTeachers", "newUsersToday", "newUsersThisWeek"}
				for _, key := range kpisToHide {
					delete(statsCopy, key)
				}
			}
			// If not allowed to see financial metrics
			if !dashboardCan(c, models.PermDashboardViewFinancialMetrics) {
				financialsToHide := []string{"dailyRevenue", "monthlyRevenue", "newSubscriptions", "cancelledSubscriptions", "pendingOrders", "pendingApprovals"}
				for _, key := range financialsToHide {
					delete(statsCopy, key)
				}
			}
			// If not allowed to see content metrics
			if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
				contentToHide := []string{"totalSubjects", "publishedCourses", "reviewCourses", "draftCourses", "archivedCourses", "totalExams", "totalResources", "activeChallenges", "moderationQueue", "totalLessons", "totalEnrollments"}
				for _, key := range contentToHide {
					delete(statsCopy, key)
				}
			}
			filtered["stats"] = statsCopy
		}
	}

	// 2. Gating revenue block
	if !dashboardCan(c, models.PermDashboardViewFinancialMetrics) {
		delete(filtered, "revenue")
		delete(filtered, "payments")
	}

	// 3. Gating users block
	if !dashboardCan(c, models.PermDashboardViewKPIs) {
		delete(filtered, "users")
		delete(filtered, "teachers")
	}

	// 4. Gating courses block
	if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
		delete(filtered, "courses")
	}

	// 5. Gating recentActivity
	if !dashboardCan(c, models.PermDashboardViewRecentActivity) {
		delete(filtered, "recentActivity")
	}

	// 6. Gating security alerts
	if !dashboardCan(c, models.PermDashboardViewAlerts) && !dashboardCan(c, models.PermDashboardViewSystemHealth) {
		delete(filtered, "security")
	}

	// 7. Gating systemAlerts
	if !dashboardCan(c, models.PermDashboardViewAlerts) {
		delete(filtered, "systemAlerts")
	}

	// 8. Gating exams
	if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
		delete(filtered, "exams")
	}

	// 9. Gating assignments
	if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
		delete(filtered, "assignments")
	}

	// 10. Gating live classes
	if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
		delete(filtered, "live")
	}

	// 11. Gating upcoming events
	if !dashboardCan(c, models.PermDashboardViewContentMetrics) {
		delete(filtered, "upcomingEvents")
	}

	return filtered
}

func getCachedAdminDashboard(ctx context.Context, cacheKey string) (map[string]interface{}, bool) {
	if cache.Redis == nil {
		return nil, false
	}
	cachedData, err := cache.Redis.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, false
	}
	var cachedResponse map[string]interface{}
	if json.Unmarshal([]byte(cachedData), &cachedResponse) != nil {
		return nil, false
	}
	return cachedResponse, true
}

func cacheAdminDashboard(cacheKey string, payload gin.H) {
	if cache.Redis == nil {
		return
	}
	dataBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	cache.Redis.Set(context.Background(), cacheKey, dataBytes, dashboardCacheTTL)
}

// dashboardPeriodStart resolves the ?time= filter into a concrete start time.
func dashboardPeriodStart(now time.Time, timeFilter string) time.Time {
	switch strings.ToLower(strings.TrimSpace(timeFilter)) {
	case "week":
		return now.AddDate(0, 0, -7)
	case "month":
		return now.AddDate(0, -1, 0)
	case "year":
		return now.AddDate(-1, 0, 0)
	default:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
}

var (
	userSubscriptionDeletedAtOnce         sync.Once
	userSubscriptionDeletedAtColumnExists bool
)

// WarmupUserSubscriptionRepository triggers the deleted_at column check at startup
// so it doesn't run on the first request. Call this after DB connection is established.
func WarmupUserSubscriptionRepository() {
	hasUserSubscriptionDeletedAtColumn()
}

func hasUserSubscriptionDeletedAtColumn() bool {
	userSubscriptionDeletedAtOnce.Do(func() {
		if db.DB != nil {
			userSubscriptionDeletedAtColumnExists = db.DB.Migrator().HasColumn(&models.UserSubscription{}, "deleted_at")
		}
	})
	return userSubscriptionDeletedAtColumnExists
}
