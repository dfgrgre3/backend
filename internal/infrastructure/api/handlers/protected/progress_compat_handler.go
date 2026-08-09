package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"

	"thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetUserCoursesProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var total int64
	if err := db.DB.Model(&models.Enrollment{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch course progress")
		return
	}

	response.Success(c, gin.H{
		"courses":        []gin.H{},
		"totalCourses":   total,
		"completed":      0,
		"inProgress":     total,
		"averagePercent": 0,
	})
}

func GetUserTimeProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	summary, err := progressQuery.GetSummary(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch time progress")
		return
	}

	response.Success(c, gin.H{
		"totalMinutes":   summary.TotalMinutes,
		"averageFocus":   summary.AverageFocus,
		"tasksCompleted": summary.TasksCompleted,
		"streakDays":     summary.StreakDays,
	})
}

func GetUserAchievementsProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	achievements, err := gamificationQuery.GetUserAchievements(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch achievements progress")
		return
	}

	response.Success(c, gin.H{
		"achievements": achievements,
		"total":        len(achievements),
	})
}

func currentUserID(c *gin.Context) (string, bool) {
	userID := c.GetString("userId")
	if userID == "" {
		return "", false
	}
	return userID, true
}
