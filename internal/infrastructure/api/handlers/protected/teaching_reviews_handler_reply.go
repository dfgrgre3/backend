package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingReplyToReview adds a reply to a course review.
func TeachingReplyToReview(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	reviewID := strings.TrimSpace(c.Param("id"))
	if reviewID == "" {
		api_response.Error(c, http.StatusBadRequest, "Review ID is required")
		return
	}

	var input struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Find the review and verify instructor owns the course
	var review models.CourseReview
	if err := database.Where("id = ?", reviewID).First(&review).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Review not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch review")
		return
	}

	// Verify instructor owns this course
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", review.SubjectID, userID).First(&subject).Error; err != nil {
		api_response.Error(c, http.StatusForbidden, "Access denied")
		return
	}

	// Log as notification (replies stored as notifications for now)
	notification := models.Notification{
		UserID:    review.UserID,
		Title:     "رد جديد على تقييمك",
		Message:   input.Text,
		Type:      models.NotificationInfo,
		Status:    "sent",
		Channels:  models.StringArray{"in-app"},
		CreatedAt: time.Now(),
	}
	if err := SafeCreate(database, &notification); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to send reply")
		return
	}

	api_response.Success(c, gin.H{
		"reply": gin.H{
			"id":     notification.ID,
			"author": "المدرب",
			"text":   input.Text,
			"date":   time.Now().Format("2006-01-02"),
		},
	})
}
