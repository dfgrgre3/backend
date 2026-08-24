package protected

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/events"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// GetEnrollmentStatus returns the current enrollment status for a user in a course
// GET /api/courses/:id/enrollment-status
func GetEnrollmentStatus(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "fetching course for enrollment status")
		return
	}

	// Check if enrolled
	var enrollment models.Enrollment
	isEnrolled := db.DB.Where("user_id = ? AND subject_id = ?", userId, subject.ID).First(&enrollment).Error == nil

	// Get lesson progress counts
	var totalLessons int64
	db.DB.Model(&models.SubTopic{}).
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("Topic.subject_id = ?", subject.ID).
		Count(&totalLessons)

	var completedLessons int64
	if isEnrolled {
		db.DB.Model(&models.LessonProgress{}).
			Where("user_id = ? AND completed = ? AND sub_topic_id IN (?)",
				userId, true,
				db.DB.Table("SubTopic").
					Select("SubTopic.id").
					Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
					Where("Topic.subject_id = ?", subject.ID),
			).
			Count(&completedLessons)
	}

	// Check if payment is required
	price, _ := subject.Price.Float64()
	paymentRequired := price > 0 && !hasPaidForSubject(userId, subject.ID)

	var progress float64
	if isEnrolled {
		progress, _ = enrollment.Progress.Float64()
	}

	// Determine enrollment status
	status := "not_enrolled"
	if isEnrolled {
		status = "enrolled"
		if totalLessons > 0 && completedLessons == totalLessons {
			status = "completed"
		}
	}

	api_response.Success(c, gin.H{
		"isEnrolled":       isEnrolled,
		"status":           status,
		"enrolledAt":       enrollment.EnrolledAt,
		"progress":         progress,
		"totalLessons":     totalLessons,
		"completedLessons": completedLessons,
		"paymentRequired":  paymentRequired,
		"price":            subject.Price,
	})
}

// UnenrollCourse removes a user's enrollment from a course
// DELETE /api/courses/:id/enroll
func UnenrollCourse(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	// Resolve subject
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "verifying subject for unenrollment")
		return
	}

	// Check if actually enrolled
	var enrollment models.Enrollment
	if err := db.DB.Where("user_id = ? AND subject_id = ?", userId, subject.ID).First(&enrollment).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "You are not enrolled in this course")
		return
	}

	// Execute unenrollment in a transaction
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Delete lesson progress for this course
		if err := tx.Where("user_id = ? AND sub_topic_id IN (?)",
			userId,
			tx.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subject.ID),
		).Delete(&models.LessonProgress{}).Error; err != nil {
			return err
		}

		// Delete enrollment
		if err := tx.Where("user_id = ? AND subject_id = ?", userId, subject.ID).Delete(&models.Enrollment{}).Error; err != nil {
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
		log.Printf("[Unenroll] Failed to unenroll user %s from course %s: %v", userId, subject.ID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to unenroll: "+err.Error())
		return
	}

	// Fire unenrollment event for gamification/analytics
	fireEnrollmentEvent(c, userId, subject.ID, string(events.EventUnenrollment))

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)
	invalidateUserListCaches(c.Request.Context(), userId)

	api_response.Success(c, gin.H{
		"success": true,
		"message": "Unenrolled successfully",
	})
}

// CompleteCourse marks a course as completed for the user
// POST /api/courses/:id/complete
func CompleteCourse(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	// Resolve subject
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "verifying subject for completion")
		return
	}

	// Check enrollment
	var enrollment models.Enrollment
	if err := db.DB.Where("user_id = ? AND subject_id = ?", userId, subject.ID).First(&enrollment).Error; err != nil {
		api_response.Error(c, http.StatusBadRequest, "You must be enrolled to complete this course")
		return
	}

	// Verify all lessons are completed
	var totalLessons int64
	db.DB.Model(&models.SubTopic{}).
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("Topic.subject_id = ?", subject.ID).
		Count(&totalLessons)

	var completedLessons int64
	db.DB.Model(&models.LessonProgress{}).
		Where("user_id = ? AND completed = ? AND sub_topic_id IN (?)",
			userId, true,
			db.DB.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subject.ID),
		).
		Count(&completedLessons)

	if totalLessons > 0 && completedLessons < totalLessons {
		api_response.Error(c, http.StatusBadRequest, "You must complete all lessons first")
		return
	}

	// Update enrollment progress to 100%
	now := time.Now()
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&enrollment).Updates(map[string]interface{}{
			"progress": 100.0,
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to complete course: "+err.Error())
		return
	}

	// Fire course completion event for gamification
	fireEnrollmentEvent(c, userId, subject.ID, string(events.EventCourseComplete))

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)
	invalidateUserListCaches(c.Request.Context(), userId)

	api_response.Success(c, gin.H{
		"success":     true,
		"message":     "Course completed successfully!",
		"completedAt": now,
	})
}

// fireEnrollmentEvent emits an enrollment-related event for gamification/analytics
func fireEnrollmentEvent(c *gin.Context, userId, subjectId, eventType string) {
	if cache.Redis == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"userId":    userId,
		"subjectId": subjectId,
	})
	event := events.AnalyticsEvent{
		ID:        uuid.New().String(),
		Type:      events.EventType(eventType),
		UserID:    userId,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Enrollment] Failed to marshal event: %v", err)
		return
	}
	if err := cache.Redis.XAdd(c.Request.Context(), &redis.XAddArgs{
		Stream: "analytics_events",
		Values: map[string]interface{}{
			"data": string(eventBytes),
		},
	}).Err(); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown command") {
			log.Printf("[Enrollment] Redis Streams (XADD) not supported (< 5.0), falling back to List: %v", err)
			_ = cache.Redis.LPush(c.Request.Context(), "analytics_events:list", string(eventBytes)).Err()
		} else {
			log.Printf("[Enrollment] Failed to publish event to Redis: %v", err)
		}
	}
}
