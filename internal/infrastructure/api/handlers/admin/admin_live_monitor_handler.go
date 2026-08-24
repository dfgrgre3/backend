package admin

import (
	"net/http"
	"strconv"
	"time"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminLive(c *gin.Context) {
	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "5"))
	if minutes <= 0 {
		minutes = 5
	}
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var users []models.User
	if err := db.DB.Where(statusQuery, models.StatusActive).Order("updated_at desc").Limit(200).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch active users")
		return
	}

	var studySessions []models.StudySession
	_ = db.DB.Where("updated_at >= ? OR start_time >= ? OR end_time >= ?", cutoff, cutoff, cutoff).Find(&studySessions).Error

	var examResults []models.ExamResult
	_ = db.DB.Where("taken_at >= ?", cutoff).Find(&examResults).Error

	summary := buildLiveActivityMaps(examResults, studySessions)

	activeUsers := make([]gin.H, 0, len(users))
	stats := struct {
		Studying   int
		TakingExam int
		Online     int
		RoleStats  map[string]int
	}{
		RoleStats: map[string]int{"students": 0, "teachers": 0, "admins": 0},
	}

	for _, user := range users {
		switch user.Role {
		case models.RoleStudent:
			stats.RoleStats["students"]++
		case models.RoleTeacher:
			stats.RoleStats["teachers"]++
		case models.RoleAdmin:
			stats.RoleStats["admins"]++
		}

		activity := determineUserLiveActivity(user, examResults, studySessions, summary)

		switch activity.Type {
		case "taking_exam":
			stats.TakingExam++
		case "studying":
			stats.Studying++
		case "online":
			stats.Online++
		}

		activeUsers = append(activeUsers, gin.H{
			"userId": user.ID,
			"user": gin.H{
				"id":     user.ID,
				"name":   firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
				"email":  user.Email,
				"role":   user.Role,
				"avatar": user.Avatar,
			},
			"lastAccessed":    activity.Time.Format(time.RFC3339),
			"currentActivity": activity.Type,
			"activityDetails": activity.Details,
			"isActive":        true,
			"sessionId":       nil,
			"ip":              nil,
			"deviceInfo":      nil,
		})
	}

	filteredUsers := filterLiveUsers(activeUsers, c.DefaultQuery("type", "all"))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"activeUsers": filteredUsers,
		"stats": gin.H{
			"totalActive": len(activeUsers),
			"studying":    stats.Studying,
			"takingExam":  stats.TakingExam,
			"online":      stats.Online,
			"byRole":      stats.RoleStats,
		},
	})
}
