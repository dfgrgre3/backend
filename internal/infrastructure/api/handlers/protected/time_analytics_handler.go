package protected

import (
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

var (
	analyticsQuery = analyticsservice.NewAnalyticsQueryService()
)

func GetTimeAnalytics(c *gin.Context) {
	userIdValue, exists := c.Get("userId")
	if !exists || userIdValue == nil {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userId := userIdValue.(string)

	result, err := analyticsQuery.GetTimeAnalytics(userId)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch time analytics")
		return
	}

	api_response.Success(c, gin.H{
		"totalStudyMinutes": result.TotalStudyMinutes,
		"totalSessions":     result.TotalSessions,
		"totalTasks":        result.TotalTasks,
		"completedTasks":    result.CompletedTasks,
		"completionRate":    result.CompletionRate,
	})
}
