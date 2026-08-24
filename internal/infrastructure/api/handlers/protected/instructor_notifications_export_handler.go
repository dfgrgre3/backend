package protected

import (
	"errors"
	"net/http"
	"strings"
	"time"
	models "thanawy-backend/internal/domain/common"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SendInstructorNotification(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorNotificationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to send instructor notification")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Instructor Update"
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = "You have a new update from the platform."
	}

	notification := models.Notification{
		UserID:    user.ID,
		Title:     title,
		Message:   message,
		Type:      models.NotificationInfo,
		Status:    "sent",
		Channels:  models.StringArray{"in-app"},
		CreatedAt: time.Now(),
	}
	if err := SafeCreate(database, &notification); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor notification")
		return
	}

	apiresponse.Success(c, gin.H{
		"id":           notification.ID,
		"instructorId": user.ID,
		"title":        title,
		"message":      message,
		"createdAt":    notification.CreatedAt,
	})
}

func BulkSendInstructorNotifications(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	var users []models.User
	err := database.Where("role = ?", models.RoleTeacher).Find(&users).Error
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructors")
		return
	}

	count := 0
	for _, user := range users {
		notification := models.Notification{
			UserID:    user.ID,
			Title:     "Bulk Instructor Update",
			Message:   "A bulk update has been sent to your account.",
			Type:      models.NotificationInfo,
			Status:    "sent",
			Channels:  models.StringArray{"in-app"},
			CreatedAt: time.Now(),
		}
		if err := SafeCreate(database, &notification); err == nil {
			count++
		}
	}

	apiresponse.Success(c, gin.H{"sent": count, "total": len(users)})
}
