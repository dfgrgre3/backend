package handlers

import (
	"net/http"
	"thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cqrs/queries"

	"github.com/gin-gonic/gin"
)

var (
	analyticsQuery = queries.NewAnalyticsQueryService()
)

func GetTimeAnalytics(c *gin.Context) {
	userIdValue, exists := c.Get("userId")
	if !exists || userIdValue == nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userId := userIdValue.(string)

	result, err := analyticsQuery.GetTimeAnalytics(userId)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch time analytics")
		return
	}

	response.Success(c, gin.H{
		"totalStudyMinutes": result.TotalStudyMinutes,
		"totalSessions":     result.TotalSessions,
		"totalTasks":        result.TotalTasks,
		"completedTasks":    result.CompletedTasks,
		"completionRate":    result.CompletionRate,
	})
}
