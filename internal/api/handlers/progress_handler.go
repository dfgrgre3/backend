package handlers

import (
	"net/http"
	"thanawy-backend/internal/cqrs/queries"

	apiresponse "thanawy-backend/internal/api/response"

	"github.com/gin-gonic/gin"
)

var (
	progressQuery = queries.NewProgressQueryService()
)

type ProgressSummary struct {
	TotalMinutes   int     `json:"totalMinutes"`
	AverageFocus   float64 `json:"averageFocus"`
	TasksCompleted int64   `json:"tasksCompleted"`
	StreakDays     int     `json:"streakDays"`
}

func GetProgressSummary(c *gin.Context) {
	userId, _ := c.Get("userId")
	uid := userId.(string)

	summary, err := progressQuery.GetSummary(uid)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to get progress summary")
		return
	}

	apiresponse.Success(c, ProgressSummary{
		TotalMinutes:   summary.TotalMinutes,
		AverageFocus:   summary.AverageFocus,
		TasksCompleted: summary.TasksCompleted,
		StreakDays:     summary.StreakDays,
	})
}

func GetWeeklyAnalytics(c *gin.Context) {
	userIdValue, exists := c.Get("userId")
	if !exists || userIdValue == nil {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userId := userIdValue.(string)

	result, err := progressQuery.GetWeeklyAnalytics(userId)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to get weekly analytics")
		return
	}

	apiresponse.Success(c, result)
}
