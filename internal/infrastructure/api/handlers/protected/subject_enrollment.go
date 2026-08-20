package protected

import (
	"log"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/events"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func EnrollCourse(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	// Ensure the authenticated account exists
	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		log.Printf("[Enrollment] authenticated user was not found in database: %q", userId)
		api_response.Error(c, http.StatusUnauthorized, "User account was not found. Please sign in again or complete registration.")
		return
	}

	// Resolve and verify subject
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "verifying subject for enrollment")
		return
	}

	// Check if user is already enrolled
	if isAlreadyEnrolled(userId, courseId) {
		api_response.Success(c, gin.H{"success": true, "message": "Already enrolled"})
		return
	}

	// Payment verification logic
	price, _ := subject.Price.Float64()
	if price > 0 {
		if !hasPaidForSubject(userId, courseId) {
			api_response.Success(c, gin.H{
				"error":           "Payment required for this course",
				"courseId":        courseId,
				"price":           price,
				"requiresPayment": true,
			})
			return
		}
	}

	if err := executeEnrollmentTransaction(userId, courseId); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	// Fire enrollment event for gamification/analytics
	fireEnrollmentEvent(c, userId, subject.ID, string(events.EventEnrollment))

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)

	api_response.Success(c, gin.H{"success": true, "message": "Enrolled successfully"})
}

func isAlreadyEnrolled(userId, subjectId string) bool {
	var enrollment models.Enrollment
	err := db.DB.Where("user_id = ? AND subject_id = ?", userId, subjectId).First(&enrollment).Error
	return err == nil
}

func hasPaidForSubject(userId, subjectId string) bool {
	var payment models.Payment
	err := db.DB.Where("user_id = ? AND subject_id = ? AND status = ?", userId, subjectId, models.PaymentCompleted).First(&payment).Error
	return err == nil
}

func executeEnrollmentTransaction(userId, subjectId string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return enrollUserInTransaction(tx, userId, subjectId)
	})
}

func enrollUserInTransaction(tx *gorm.DB, userId, subjectId string) error {
	enrollment := models.Enrollment{
		UserID:     userId,
		SubjectID:  subjectId,
		EnrolledAt: time.Now(),
	}

	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollment)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return tx.Model(&models.Subject{}).
			Where(idQuery, subjectId).
			Update("enrolled_count", gorm.Expr("enrolled_count + 1")).Error
	}
	return nil
}

func GetUserSubjects(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var enrollments []models.Enrollment
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Limit(limit).
		Offset(offset).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	// For the frontend useTimeData.ts which expects { id, subject: "MATH" }
	type subjectResponse struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}

	response := []subjectResponse{}
	for _, e := range enrollments {
		if e.Subject.ID != "" {
			response = append(response, subjectResponse{
				ID:      e.ID,
				Subject: e.Subject.Name,
			})
		}
	}

	api_response.Success(c, response)
}

func GetMyCourses(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	var enrollments []models.Enrollment
	if err := readDB.
		Preload("Subject").
		Where("user_id = ?", userId).
		Order("updated_at DESC").
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	courses := make([]gin.H, 0, len(enrollments))
	for _, enrollment := range enrollments {
		subject := enrollment.Subject
		if subject.ID == "" {
			continue
		}

		title := subject.Name
		if subject.NameAr != nil && *subject.NameAr != "" {
			title = *subject.NameAr
		}

		courses = append(courses, gin.H{
			"id":             subject.ID,
			"enrollmentId":   enrollment.ID,
			"title":          title,
			"name":           subject.Name,
			"nameAr":         subject.NameAr,
			"description":    subject.Description,
			"thumbnailUrl":   subject.ThumbnailUrl,
			"progress":       enrollment.Progress,
			"enrolled":       true,
			"lastAccessedAt": enrollment.UpdatedAt,
			"enrolledAt":     enrollment.EnrolledAt,
			"subject":        subject.Code,
			"rating":         subject.Rating,
			"level":          subject.Level,
		})
	}

	api_response.Success(c, gin.H{
		"courses": courses,
		"data": gin.H{
			"courses": courses,
		},
	})
}
