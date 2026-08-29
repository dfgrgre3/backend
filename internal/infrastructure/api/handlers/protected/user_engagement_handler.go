package protected

import (
	"net/http"
	"strconv"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminEnrollUser manually enrolls a user in a course, with an optional
// payment-bypass flag (isFree) for admin use.
func AdminEnrollUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		CourseID string `json:"courseId" binding:"required"`
		IsFree   bool   `json:"isFree"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, req.CourseID).First(&subject).Error; err != nil {
		handleSubjectError(c, req.CourseID, err, "verifying course for manual enrollment")
		return
	}

	if isAlreadyEnrolled(userID, subject.ID) {
		api_response.Success(c, gin.H{"success": true, "alreadyEnrolled": true, "message": "User is already enrolled"})
		return
	}

	if !req.IsFree {
		price, _ := subject.Price.Float64()
		if price > 0 && !hasPaidForSubject(userID, subject.ID) {
			api_response.Error(c, http.StatusBadRequest, "User has not paid for this course. Use isFree=true to bypass payment.")
			return
		}
	}

	if err := executeEnrollmentTransaction(userID, subject.ID); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to enroll", err)
		return
	}

	api_response.Success(c, gin.H{"success": true, "message": "User enrolled successfully"})
}

// GetUserVideoEngagement returns video watch progress records for a user.
func GetUserVideoEngagement(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 100
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var progressRecords []models.LessonProgress
	if err := db.DB.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&progressRecords).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch video engagement")
		return
	}

	videos := make([]gin.H, 0, len(progressRecords))
	totalWatchSeconds := 0
	for _, p := range progressRecords {
		totalWatchSeconds += p.TimeSpentSeconds
		videos = append(videos, gin.H{
			"lessonId":            p.LessonID,
			"timeSpentSeconds":    p.TimeSpentSeconds,
			"timeSpentMinutes":    p.TimeSpentSeconds / 60,
			"completed":           p.Completed,
			"status":              string(p.Status),
			"lastWatchedPosition": p.LastWatchedPosition,
		})
	}

	api_response.Success(c, gin.H{
		"userId":            userID,
		"totalVideos":       len(videos),
		"totalWatchSeconds": totalWatchSeconds,
		"totalWatchMinutes": totalWatchSeconds / 60,
		"videos":            videos,
	})
}
