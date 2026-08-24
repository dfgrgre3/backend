package protected

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// TeachingGetActivities returns recent activities for the instructor.
func TeachingGetActivities(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 20
	var activities []models.ActivityLog
	if err := database.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error; err != nil {
		// Silently return empty
	}

	type ActivityItem struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		MessageAr   string `json:"messageAr"`
		MessageEn   string `json:"messageEn"`
		Time        string `json:"time"`
		StudentName string `json:"studentName,omitempty"`
		CourseTitle string `json:"courseTitle,omitempty"`
		Rating      int    `json:"rating,omitempty"`
	}

	items := make([]ActivityItem, 0, len(activities))
	for _, a := range activities {
		items = append(items, ActivityItem{
			ID:        a.ID,
			Type:      a.Action,
			MessageAr: a.Resource,
			MessageEn: a.Resource,
			Time:      formatRelativeTime(a.CreatedAt),
		})
	}

	api_response.Success(c, gin.H{"activities": items})
}

// TeachingGetNotifications returns notifications for the instructor.
func TeachingGetNotifications(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 30
	var notifications []models.Notification
	if err := database.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch notifications")
		return
	}

	type NotifItem struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
		Time  string `json:"time"`
		Read  bool   `json:"read"`
		Type  string `json:"type"`
	}

	items := make([]NotifItem, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, NotifItem{
			ID:    n.ID,
			Title: n.Title,
			Body:  n.Message,
			Time:  formatRelativeTime(n.CreatedAt),
			Read:  n.IsRead,
			Type:  "system",
		})
	}

	api_response.Success(c, gin.H{"notifications": items})
}

// TeachingMarkNotificationRead marks a notification as read.
func TeachingMarkNotificationRead(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	notifID := strings.TrimSpace(c.Param("id"))
	if notifID == "" {
		api_response.Error(c, http.StatusBadRequest, "Notification ID is required")
		return
	}

	database.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notifID, userID).
		Update("is_read", true)

	api_response.Success(c, gin.H{"marked": true})
}

// TeachingMarkAllNotificationsRead marks all notifications as read.
func TeachingMarkAllNotificationsRead(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	database.Model(&models.Notification{}).
		Where("user_id = ?", userID).
		Update("is_read", true)

	api_response.Success(c, gin.H{"marked": true})
}

// TeachingApplyForInstructor handles instructor application.
func TeachingApplyForInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Experience string `json:"experience"`
		Bio        string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Generate application code
	bytes := make([]byte, 4)
	rand.Read(bytes)
	code := "TOLO-TCHR-" + strings.ToUpper(hex.EncodeToString(bytes))

	// Update user with instructor application data
	bio := strings.TrimSpace(input.Bio)
	updates := map[string]interface{}{
		"bio":               &bio,
		"instructor_status": "PENDING",
		"experience_years":  input.Experience,
	}
	if input.Name != "" {
		name := input.Name
		updates["name"] = &name
	}

	database.Model(&models.User{}).Where("id = ?", userID).Updates(updates)

	api_response.Success(c, gin.H{
		"code":    code,
		"email":   input.Email,
		"message": "Your application has been submitted for review. You will receive an email once approved.",
	})
}
