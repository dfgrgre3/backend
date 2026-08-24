package protected

import (
	"math"
	"net/http"
	"strconv"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ManualEnroll is an admin handler to manually enroll a user in a course
// POST /api/admin/courses/enroll
func ManualEnroll(c *gin.Context) {
	var input struct {
		UserID   string `json:"userId" binding:"required"`
		CourseID string `json:"courseId" binding:"required"`
		IsFree   bool   `json:"isFree"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	// Verify user exists
	var user models.User
	if err := db.DB.First(&user, "id = ?", input.UserID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	// Resolve course
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, input.CourseID).First(&subject).Error; err != nil {
		handleSubjectError(c, input.CourseID, err, "verifying course for manual enrollment")
		return
	}

	// Check if already enrolled
	if isAlreadyEnrolled(input.UserID, subject.ID) {
		api_response.Success(c, gin.H{"success": true, "message": "User is already enrolled"})
		return
	}

	// If not free, check payment
	price, _ := subject.Price.Float64()
	if !input.IsFree && price > 0 {
		if !hasPaidForSubject(input.UserID, subject.ID) {
			api_response.Error(c, http.StatusBadRequest, "User has not paid for this course. Use isFree=true to bypass payment.")
			return
		}
	}

	if err := executeEnrollmentTransaction(input.UserID, subject.ID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	analyticsservice.GetAuditService().LogAsync(input.UserID, "admin.enroll", "enrollment", subject.ID,
		map[string]interface{}{"adminId": c.GetString("userId")}, c.ClientIP(), c.Request.UserAgent())

	api_response.Success(c, gin.H{"success": true, "message": "User enrolled successfully"})
}

// UnenrollUser is an admin handler to unenroll a user from a course
// POST /api/admin/courses/unenroll
func UnenrollUser(c *gin.Context) {
	var input struct {
		UserID   string `json:"userId" binding:"required"`
		CourseID string `json:"courseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	// Resolve course
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, input.CourseID).First(&subject).Error; err != nil {
		handleSubjectError(c, input.CourseID, err, "verifying course for admin unenrollment")
		return
	}

	// Check enrollment
	var enrollment models.Enrollment
	if err := db.DB.Where("user_id = ? AND subject_id = ?", input.UserID, subject.ID).First(&enrollment).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User is not enrolled in this course")
		return
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Delete lesson progress
		if err := tx.Where("user_id = ? AND sub_topic_id IN (?)",
			input.UserID,
			tx.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subject.ID),
		).Delete(&models.LessonProgress{}).Error; err != nil {
			return err
		}

		// Delete enrollment
		if err := tx.Where("user_id = ? AND subject_id = ?", input.UserID, subject.ID).Delete(&models.Enrollment{}).Error; err != nil {
			return err
		}

		// Decrement enrolled count
		if err := tx.Model(&models.Subject{}).
			Where("id = ?", subject.ID).
			Update("enrolled_count", gorm.Expr("GREATEST(enrolled_count - 1, 0)")).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to unenroll user: "+err.Error())
		return
	}

	analyticsservice.GetAuditService().LogAsync(input.UserID, "admin.unenroll", "enrollment", subject.ID,
		map[string]interface{}{"adminId": c.GetString("userId")}, c.ClientIP(), c.Request.UserAgent())

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)
	invalidateUserListCaches(c.Request.Context(), input.UserID)

	api_response.Success(c, gin.H{"success": true, "message": "User unenrolled successfully"})
}

// GetCourseEnrollments returns all enrollments with pagination for admin
// GET /api/admin/courses/enrollments
func GetCourseEnrollments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	courseId := c.Query("courseId")
	userId := c.Query("userId")

	var total int64
	query := db.DB.Model(&models.Enrollment{}).
		Preload("User").
		Preload("Subject")

	if courseId != "" {
		query = query.Where("subject_id = ?", courseId)
	}
	if userId != "" {
		query = query.Where("user_id = ?", userId)
	}

	query.Count(&total)

	var enrollments []models.Enrollment
	if err := query.Order("enrolled_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	api_response.Success(c, gin.H{
		"enrollments": enrollments,
		"pagination": api_response.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
		},
	})
}
